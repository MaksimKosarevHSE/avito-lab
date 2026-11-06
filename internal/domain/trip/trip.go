package trip

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func validCoordinate(value, minValue, maxValue float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minValue && value <= maxValue
}

func NewTrip(
	id uuid.UUID,
	userID uuid.UUID,
	driverID uuid.UUID,
	startLatitude float64,
	startLongitude float64,
	endLatitude float64,
	endLongitude float64,
	price int64,
	createdAt time.Time,
) (*Trip, error) {
	switch {
	case id == uuid.Nil:
		return nil, fmt.Errorf("%w: ID is required", ErrInvalidTrip)
	case userID == uuid.Nil:
		return nil, fmt.Errorf("%w: user ID is required", ErrInvalidTrip)
	case driverID == uuid.Nil:
		return nil, fmt.Errorf("%w: driver ID is required", ErrInvalidTrip)
	case !validCoordinate(startLatitude, -90, 90):
		return nil, fmt.Errorf("%w: start latitude must be between -90 and 90", ErrInvalidTrip)
	case !validCoordinate(startLongitude, -180, 180):
		return nil, fmt.Errorf("%w: start longitude must be between -180 and 180", ErrInvalidTrip)
	case !validCoordinate(endLatitude, -90, 90):
		return nil, fmt.Errorf("%w: end latitude must be between -90 and 90", ErrInvalidTrip)
	case !validCoordinate(endLongitude, -180, 180):
		return nil, fmt.Errorf("%w: end longitude must be between -180 and 180", ErrInvalidTrip)
	case price < 0:
		return nil, fmt.Errorf("%w: price must not be negative", ErrInvalidTrip)
	case createdAt.IsZero():
		return nil, fmt.Errorf("%w: creation time is required", ErrInvalidTrip)
	}

	return &Trip{
		ID:             id,
		UserID:         userID,
		DriverID:       driverID,
		StartLatitude:  startLatitude,
		StartLongitude: startLongitude,
		EndLatitude:    endLatitude,
		EndLongitude:   endLongitude,
		Price:          price,
		Status:         StatusActive,
		StartedAt:      createdAt,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}, nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}

	t.Status = StatusCompleted
	t.FinishedAt = &at
	t.UpdatedAt = at

	return nil
}

type StatusHistory struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus *Status
	ToStatus   Status
	Reason     *string
	ChangedAt  time.Time
}

type IdempotencyRecord struct {
	Key                uuid.UUID
	FingerprintVersion int16
	RequestFingerprint []byte
	TripID             uuid.UUID
}
