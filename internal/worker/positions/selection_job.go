package positions

import (
	"context"
	"log/slog"
	"time"
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
}

func NewSelectionJob(
	positionStaleAfter time.Duration,
	positionScanBatchSize int32,
	positionCheckInterval time.Duration,
	tripBatchSelector TripBatchSelector,
	tripEnqueuer TripEnqueuer,
	logger *slog.Logger,
) *SelectionJob {
	return &SelectionJob{
		positionStaleAfter:    positionStaleAfter,
		positionScanBatchSize: positionScanBatchSize,
		positionCheckInterval: positionCheckInterval,
		tripEnqueuer:          tripEnqueuer,
		tripBatchSelector:     tripBatchSelector,
		logger:                logger,
	}
}

func (j *SelectionJob) Run(ctx context.Context) {
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
	var nextPtr *FetchForPushPtr = nil
	stopped := false
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
