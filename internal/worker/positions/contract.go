package positions

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type StaleCheckResult string

const (
	StaleCheckResultQueued    StaleCheckResult = "queued"
	StaleCheckResultDuplicate StaleCheckResult = "duplicate"
	StaleCheckResultDropped   StaleCheckResult = "dropped"
)

type PositionRequestResult string

const (
	PositionRequestResultSuccess PositionRequestResult = "success"
	PositionRequestResultError   PositionRequestResult = "error"
	PositionRequestResultDropped PositionRequestResult = "dropped"
)

type SelectedTrip struct {
	TripID           uuid.UUID
	DriverID         uuid.UUID
	LastTrackedAt    time.Time
	CheckSpanContext trace.SpanContext
}
type FetchForPushPtr struct {
	LastTrackedAt time.Time
	TripID        uuid.UUID
}

type FetchForPushResult struct {
	SelectedTrips []SelectedTrip
	NextPtr       *FetchForPushPtr
}

type TripBatchSelector interface {
	FetchForPushQuery(
		ctx context.Context,
		positionStaleAfter time.Duration,
		positionScanBatchSize int32,
		ptr *FetchForPushPtr, // optional
	) (FetchForPushResult, error)
}

type JobPositionMetrics interface {
	AddStaleCheckResult(ctx context.Context, result StaleCheckResult)
	AddPositionRequestResult(ctx context.Context, result PositionRequestResult, duration time.Duration)
	RecordRowsRead(ctx context.Context, cnt int64)
}

type WorkerPositionMetrics interface {
	IncrementActiveWorkers()
	DecrementActiveWorkers()
	AddPositionRequestResult(ctx context.Context, result PositionRequestResult, duration time.Duration)
}
