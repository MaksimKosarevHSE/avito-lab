//go:generate go tool mockgen -source=${GOFILE} -destination=mocks_test.go -package=${GOPACKAGE}_test

package create

import (
	"context"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
)

type TripRepository interface {
	Create(ctx context.Context, value trip.Trip) error
	ClaimIdempotencyKey(ctx context.Context, record trip.IdempotencyRecord, ttl time.Duration) (bool, error)
	AddStatusHistory(ctx context.Context, history trip.StatusHistory) error
	GetIdempotencyKey(
		ctx context.Context,
		key uuid.UUID,
	) (trip.IdempotencyRecord, error)
	GetByID(
		ctx context.Context,
		id uuid.UUID,
	) (trip.Trip, error)
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
