package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type TripSupplier interface {
	Take() (SelectedTrip, bool)
	Complete(tripID uuid.UUID)
	Close()
}

type PositionPusher interface {
	PushPosition(ctx context.Context, tripID, driverID uuid.UUID) error
}

type WorkerPool struct {
	workersCount    int32
	tripSupplier    TripSupplier
	positionPusher  PositionPusher
	logger          *slog.Logger
	positionMetrics WorkerPositionMetrics
	tracer          trace.Tracer
}

func NewWorkerPool(workersCount int32,
	tripSupplier TripSupplier,
	positionPusher PositionPusher,
	logger *slog.Logger,
	workerPositionMetrics WorkerPositionMetrics,
	tracer trace.Tracer,
) *WorkerPool {
	return &WorkerPool{
		workersCount:    workersCount,
		tripSupplier:    tripSupplier,
		positionPusher:  positionPusher,
		logger:          logger.With("component", "position-worker-pool"),
		positionMetrics: workerPositionMetrics,
		tracer:          tracer,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	wp.logger.InfoContext(ctx, "component started")
	defer wp.logger.InfoContext(context.WithoutCancel(ctx), "component stopped")

	var wg sync.WaitGroup
	wg.Add(int(wp.workersCount))
	for i := range wp.workersCount {
		go wp.worker(ctx, int(i), &wg)
	}
	result := make(chan struct{})

	go func() {
		wg.Wait()
		close(result)
	}()

	select {
	case <-ctx.Done():
		wp.tripSupplier.Close()
		<-result
	case <-result:
	}
}

func (wp *WorkerPool) Stop() {
	wp.tripSupplier.Close()
}

func (wp *WorkerPool) worker(ctx context.Context, _ int, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		trip, ok := wp.tripSupplier.Take()
		if !ok {
			return
		}
		if ctx.Err() != nil {
			return
		}
		wp.pushPosition(ctx, trip)
	}
}

func (wp *WorkerPool) pushPosition(ctx context.Context, trip SelectedTrip) {
	spanOptions := []trace.SpanStartOption{
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("trip.id", trip.TripID.String()),
			attribute.String("driver.id", trip.DriverID.String()),
		),
	}
	if trip.CheckSpanContext.IsValid() {
		spanOptions = append(spanOptions, trace.WithLinks(trace.Link{SpanContext: trip.CheckSpanContext}))
	}

	ctx, span := wp.tracer.Start(ctx, "positions.process_request", spanOptions...)
	defer span.End()

	wp.positionMetrics.IncrementActiveWorkers()
	defer wp.positionMetrics.DecrementActiveWorkers()
	defer wp.tripSupplier.Complete(trip.TripID)

	reqResult := PositionRequestResultSuccess

	startedAt := time.Now()
	err := wp.positionPusher.PushPosition(ctx, trip.TripID, trip.DriverID)
	duration := time.Since(startedAt)

	if err != nil {
		reqResult = PositionRequestResultError
		span.RecordError(err)
		span.SetStatus(codes.Error, "push position failed")
		wp.logger.WarnContext(ctx, "push position failed", "trip_id", trip.TripID, "error", err)
	}
	span.SetAttributes(attribute.String("tripgo.position_request.result", string(reqResult)))

	wp.positionMetrics.AddPositionRequestResult(
		ctx, reqResult,
		duration,
	)
}
