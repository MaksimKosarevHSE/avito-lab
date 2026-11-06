package idempotency

import (
	"context"
	"log/slog"
	"time"
)

const cleanupBatchSize = 100

type Cleaner interface {
	DeleteExpiredIdempotencyKeys(ctx context.Context, limit int) (int64, error)
}

type CleanupJob struct {
	cleaner  Cleaner
	interval time.Duration
	logger   *slog.Logger
}

func NewCleanupJob(cleaner Cleaner, interval time.Duration, logger *slog.Logger) *CleanupJob {
	return &CleanupJob{
		cleaner:  cleaner,
		interval: interval,
		logger:   logger.With("component", "idempotency-cleanup"),
	}
}

func (j *CleanupJob) Run(ctx context.Context) {
	for {
		j.cleanup(ctx)

		timer := time.NewTimer(j.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (j *CleanupJob) cleanup(ctx context.Context) {
	deleted, err := j.cleaner.DeleteExpiredIdempotencyKeys(ctx, cleanupBatchSize)
	if err != nil {
		if ctx.Err() == nil {
			j.logger.ErrorContext(ctx, "delete expired idempotency keys", "error", err)
		}
		return
	}

	if deleted > 0 {
		j.logger.DebugContext(ctx, "expired idempotency keys deleted", "count", deleted)
	}
}
