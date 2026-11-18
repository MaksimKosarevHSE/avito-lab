package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	return r.getByID(ctx, id, false)
}

func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	return r.getByID(ctx, id, true)
}

func (r *Repository) getByID(
	ctx context.Context,
	id uuid.UUID,
	forUpdate bool,
) (domaintrip.Trip, error) {
	queryBuilder := r.builder.
		Select(tripColumns...).
		From(tripsTable).
		Where(sq.Eq{idColumn: id})
	if forUpdate {
		queryBuilder = queryBuilder.Suffix("FOR UPDATE")
	}

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	var result domaintrip.Trip
	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		var scanErr error
		result, scanErr = scanTrip(executor.QueryRow(queryCtx, query, args...))
		return scanErr
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", domaintrip.ErrTripNotFound)
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return result, nil
}
