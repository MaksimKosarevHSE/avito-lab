package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

func (r *Repository) Update(ctx context.Context, value domaintrip.Trip) error {
	query, args, err := r.builder.
		Update(tripsTable).
		Set(statusColumn, string(value.Status)).
		Set(finishedAtColumn, value.FinishedAt).
		Set(updatedAtColumn, value.UpdatedAt).
		Where(sq.Eq{idColumn: value.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update trip query: %w", err)
	}

	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		commandTag, execErr := executor.Exec(queryCtx, query, args...)
		if execErr != nil {
			return execErr
		}
		if commandTag.RowsAffected() == 0 {
			return domaintrip.ErrTripNotFound
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update trip: %w", err)
	}

	return nil
}

func (r *Repository) UpdateLastPositionAt(ctx context.Context, id uuid.UUID, recordedAt time.Time) error {
	query, args, err := r.builder.
		Update(tripsTable).
		Set(lastPositionAtColumn, sq.Expr(
			`CASE
					WHEN last_position_at IS NULL 
					OR last_position_at < ?
					THEN ?
				ELSE
					last_position_at
				END`,
			recordedAt,
			recordedAt,
		)).
		Where(sq.Eq{idColumn: id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update last trip position query: %w", err)
	}

	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		commandTag, execErr := executor.Exec(queryCtx, query, args...)
		if execErr != nil {
			return execErr
		}
		if commandTag.RowsAffected() == 0 {
			return domaintrip.ErrTripNotFound
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("update last trip position : %w", err)
	}

	return nil
}
