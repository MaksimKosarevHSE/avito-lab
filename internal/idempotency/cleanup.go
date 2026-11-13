package idempotency

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const cleanupBatchSize = 100

type Cleaner interface {
	DeleteExpiredIdempotencyKeys(ctx context.Context, limit int) (int64, error)
}

type CleanupJob struct {
	cleaner  Cleaner
	interval time.Duration
	logger   *slog.Logger
	tracer   trace.Tracer
}

func NewCleanupJob(
	cleaner Cleaner,
	interval time.Duration,
	logger *slog.Logger,
	tracer trace.Tracer,
) *CleanupJob {
	return &CleanupJob{
		cleaner:  cleaner,
		interval: interval,
		logger:   logger.With("component", "idempotency-cleanup"),
		tracer:   tracer,
	}
}

func (j *CleanupJob) Run(ctx context.Context) {
	j.logger.InfoContext(ctx, "component started")
	defer j.logger.InfoContext(context.WithoutCancel(ctx), "component stopped")

	for {
		cleanupCtx, span := j.tracer.Start(ctx, "idempotency cleanup")
		j.cleanup(cleanupCtx)
		span.End()

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
