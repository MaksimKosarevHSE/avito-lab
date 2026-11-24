package positions

import (
	"context"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/telemetry/position"
	"github.com/google/uuid"
)

type SelectedTrip struct {
	TripID        uuid.UUID
	DriverID      uuid.UUID
	LastTrackedAt time.Time
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
	AddStaleCheckResult(ctx context.Context, result position.StaleCheckResult)
	RecordRowsRead(ctx context.Context, cnt int64)
}

type WorkerPositionMetrics interface {
	IncrementActiveWorkers()
	DecrementActiveWorkers()
	AddPositionRequestResult(ctx context.Context, result position.PositionRequestResult, duration time.Duration)
}
