package positions

import (
	"time"

	"github.com/google/uuid"
)

type SelectedTrip struct {
	TripID           uuid.UUID
	DriverID         uuid.UUID
	DontTrackedAfter time.Time
}
