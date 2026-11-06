package get

import (
	"context"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
)

type TripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
