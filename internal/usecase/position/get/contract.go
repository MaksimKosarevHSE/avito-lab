package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

type positionRepository interface {
	GetListByTripID(ctx context.Context, id uuid.UUID) ([]position.Position, error)
}

type tripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
