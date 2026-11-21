package positions

import (
	"context"
	"time"

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
