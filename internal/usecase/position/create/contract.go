package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

type positionRepository interface {
	Create(ctx context.Context, position position.Position) (position.Position, error)
}

type tripRepository interface {
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	UpdateLastPositionAt(ctx context.Context, id uuid.UUID, recordedAt time.Time) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type positionPushCanceller interface {
	Cancel(tripID uuid.UUID) bool
}

type positionSaveMetrics interface {
	AddPositionSaved(ctx context.Context)
}
