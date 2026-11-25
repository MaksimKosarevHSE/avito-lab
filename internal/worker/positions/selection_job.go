package positions

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/telemetry/position"
)

type TripEnqueuer interface {
	Enqueue(trip SelectedTrip) EnqueueResult
}

type SelectionJob struct {
	positionStaleAfter    time.Duration
	positionCheckInterval time.Duration
	positionScanBatchSize int32

	clock             Clock
	tripBatchSelector TripBatchSelector
	tripEnqueuer      TripEnqueuer
	logger            *slog.Logger
	positionMetrics   JobPositionMetrics
	tracer            trace.Tracer
}

func NewSelectionJob(
	positionStaleAfter time.Duration,
	positionScanBatchSize int32,
	positionCheckInterval time.Duration,
	clock Clock,
	tripBatchSelector TripBatchSelector,
	tripEnqueuer TripEnqueuer,
	logger *slog.Logger,
	positionMetrics JobPositionMetrics,
	tracer trace.Tracer,
) *SelectionJob {
	return &SelectionJob{
		positionStaleAfter:    positionStaleAfter,
		positionScanBatchSize: positionScanBatchSize,
		positionCheckInterval: positionCheckInterval,
		clock:                 clock,
		tripEnqueuer:          tripEnqueuer,
		tripBatchSelector:     tripBatchSelector,
		logger:                logger.With("component", "position-selection"),
		positionMetrics:       positionMetrics,
		tracer:                tracer,
	}
}

func (j *SelectionJob) Run(ctx context.Context) {
	j.logger.InfoContext(ctx, "component started")
	defer j.logger.InfoContext(context.WithoutCancel(ctx), "component stopped")

	ticker := j.clock.NewTicker(j.positionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			j.fetchForPushQuery(ctx)
		}
	}
}

func (j *SelectionJob) fetchForPushQuery(ctx context.Context) {
	ctx, span := j.tracer.Start(
		ctx,
		"positions.stale_check",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	rowsReadCnt := 0
	defer func() {
		span.SetAttributes(attribute.Int("tripgo.stale_checks.rows_read", rowsReadCnt))
		j.positionMetrics.RecordRowsRead(ctx, int64(rowsReadCnt))
	}()

	stopped := false
	var nextPtr *FetchForPushPtr = nil

	for !stopped {
		result, err := j.tripBatchSelector.FetchForPushQuery(
			ctx,
			j.positionStaleAfter,
			j.positionScanBatchSize,
			nextPtr,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "fetch stale trips failed")
			if ctx.Err() == nil {
				j.logger.ErrorContext(ctx, "fetch for push query", "error", err)
			}
			return
		}

		rowsReadCnt += len(result.SelectedTrips)

		for _, trip := range result.SelectedTrips {
			trip.CheckSpanContext = span.SpanContext()
			enqueueResult := j.tripEnqueuer.Enqueue(trip)
			switch enqueueResult.Status {
			case PushStatusClosed:
				return
			case PushStatusDropped:
				j.logger.WarnContext(
					ctx,
					"position request dropped because queue is full",
					"trip_id", trip.TripID,
				)
				j.positionMetrics.AddPositionRequestResult(
					ctx,
					position.PositionRequestResultDropped,
					0,
				)
				stopped = true
			}

			if enqueueResult.Removed != nil {
				j.logger.WarnContext(
					ctx,
					"position request evicted from full queue",
					"trip_id", enqueueResult.Removed.TripID,
					"replacement_trip_id", trip.TripID,
				)
				j.positionMetrics.AddStaleCheckResult(ctx, position.StaleCheckResultDropped)
				j.positionMetrics.AddPositionRequestResult(
					ctx,
					position.PositionRequestResultDropped,
					0,
				)
			}

			if staleCheckResult, ok := toStaleCheckResult(enqueueResult.Status); ok {
				j.positionMetrics.AddStaleCheckResult(ctx, staleCheckResult)
			}

			if stopped {
				break
			}
		}
		nextPtr = result.NextPtr

		if len(result.SelectedTrips) < int(j.positionScanBatchSize) {
			stopped = true
		}
	}
}

func toStaleCheckResult(status EnqueueStatus) (position.StaleCheckResult, bool) {
	switch status {
	case PushStatusQueued:
		return position.StaleCheckResultQueued, true
	case PushStatusDuplicate:
		return position.StaleCheckResultDuplicate, true
	case PushStatusDropped:
		return position.StaleCheckResultDropped, true
	default:
		return "", false
	}
}
