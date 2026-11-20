package position

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	CreatedAt  time.Time
}

func NewPosition(
	tripID uuid.UUID,
	latitude float64,
	longitude float64,
	recordedAt time.Time,
	createdAt time.Time,
) (*Position, error) {
	switch {
	case tripID == uuid.Nil:
		return nil, fmt.Errorf("%w: trip id is required", ErrInvalidPosition)
	case createdAt.IsZero():
		return nil, fmt.Errorf("%w: creation time is required", ErrInvalidPosition)
	case recordedAt.IsZero():
		return nil, fmt.Errorf("%w: recording time is required", ErrInvalidPosition)
	case !validCoordinate(latitude, -90, 90):
		return nil, fmt.Errorf("%w: latitude must be between -90 and 90", ErrInvalidPosition)
	case !validCoordinate(longitude, -180, 180):
		return nil, fmt.Errorf("%w: longitude must be between -180 and 180", ErrInvalidPosition)
	}

	return &Position{
		TripID:     tripID,
		Latitude:   latitude,
		Longitude:  longitude,
		RecordedAt: recordedAt.UTC(),
		CreatedAt:  createdAt.UTC(),
	}, nil
}

func validCoordinate(value, minValue, maxValue float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minValue && value <= maxValue
}
