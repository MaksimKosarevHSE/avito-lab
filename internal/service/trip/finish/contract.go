package finish

import (
	"context"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
)

type TripRepository interface {
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	Update(ctx context.Context, value trip.Trip) error
	AddStatusHistory(ctx context.Context, history trip.StatusHistory) error
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
