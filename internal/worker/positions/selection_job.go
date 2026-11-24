package positions

import (
	"context"
	"log/slog"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/telemetry/position"
)

type TripEnqueuer interface {
	Enqueue(trip SelectedTrip) EnqueueResult
}

type SelectionJob struct {
	positionStaleAfter    time.Duration
	positionCheckInterval time.Duration
	positionScanBatchSize int32

	tripBatchSelector TripBatchSelector
	tripEnqueuer      TripEnqueuer
	logger            *slog.Logger
	positionMetrics   JobPositionMetrics
}

func NewSelectionJob(
	positionStaleAfter time.Duration,
	positionScanBatchSize int32,
	positionCheckInterval time.Duration,
	tripBatchSelector TripBatchSelector,
	tripEnqueuer TripEnqueuer,
	logger *slog.Logger,
	positionMetrics JobPositionMetrics,
) *SelectionJob {
	return &SelectionJob{
		positionStaleAfter:    positionStaleAfter,
		positionScanBatchSize: positionScanBatchSize,
		positionCheckInterval: positionCheckInterval,
		tripEnqueuer:          tripEnqueuer,
		tripBatchSelector:     tripBatchSelector,
		logger:                logger.With("component", "position-selection"),
		positionMetrics:       positionMetrics,
	}
}

func (j *SelectionJob) Run(ctx context.Context) {
	j.logger.InfoContext(ctx, "component started")
	defer j.logger.InfoContext(context.WithoutCancel(ctx), "component stopped")

	ticker := time.NewTicker(j.positionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.fetchForPushQuery(ctx)
		}
	}
}

func (j *SelectionJob) fetchForPushQuery(ctx context.Context) {
	rowsReadCnt := 0
	defer func() {
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
			if ctx.Err() == nil {
				j.logger.ErrorContext(ctx, "fetch for push query", "error", err)
			}
			return
		}

		rowsReadCnt += len(result.SelectedTrips)

		for _, trip := range result.SelectedTrips {
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
