package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/telemetry/position"
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
}

func NewWorkerPool(workersCount int32,
	tripSupplier TripSupplier,
	positionPusher PositionPusher,
	logger *slog.Logger,
	workerPositionMetrics WorkerPositionMetrics,
) *WorkerPool {
	return &WorkerPool{
		workersCount:    workersCount,
		tripSupplier:    tripSupplier,
		positionPusher:  positionPusher,
		logger:          logger.With("component", "position-worker-pool"),
		positionMetrics: workerPositionMetrics,
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
		wp.pushPosition(ctx, trip.TripID, trip.DriverID)
	}
}

func (wp *WorkerPool) pushPosition(ctx context.Context, tripID, driverID uuid.UUID) {
	wp.positionMetrics.IncrementActiveWorkers()
	defer wp.positionMetrics.DecrementActiveWorkers()
	defer wp.tripSupplier.Complete(tripID)

	reqResult := position.PositionRequestResultSuccess

	startedAt := time.Now()
	err := wp.positionPusher.PushPosition(ctx, tripID, driverID)
	duration := time.Since(startedAt)

	if err != nil {
		reqResult = position.PositionRequestResultError
		wp.logger.WarnContext(ctx, "push position failed", "trip_id", tripID, "error", err)
	}

	wp.positionMetrics.AddPositionRequestResult(
		ctx, reqResult,
		duration,
	)
}
