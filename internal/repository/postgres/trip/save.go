package trip

import (
	"context"
	"errors"
	"fmt"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Repository) Create(ctx context.Context, value domaintrip.Trip) error {
	query, args, err := r.builder.
		Insert(tripsTable).
		Columns(tripColumns...).
		Values(
			value.ID,
			value.UserID,
			value.DriverID,
			value.StartLatitude,
			value.StartLongitude,
			value.EndLatitude,
			value.EndLongitude,
			value.Price,
			string(value.Status),
			value.StartedAt,
			value.FinishedAt,
			value.CreatedAt,
			value.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		_, execErr := executor.Exec(queryCtx, query, args...)
		return execErr
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == activeTripPerDriverConstraint {
			return fmt.Errorf("create trip: %w", domaintrip.ErrDriverBusy)
		}

		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}
