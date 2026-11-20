package position

import (
	"context"
	"fmt"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

func (r *Repository) GetListByTripID(ctx context.Context, tripId uuid.UUID) ([]position.Position, error) {
	query, args, err := r.builder.
		Select(positionColumns...).
		From(positionsTable).
		Where(squirrel.Eq{tripIDColumn: tripId}).
		OrderBy(
			recordedAtColumn+" ASC",
			idColumn+" ASC",
		).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list positions query: %w", err)
	}

	result := make([]position.Position, 0)

	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		rows, queryErr := executor.Query(queryCtx, query, args...)
		if queryErr != nil {
			return queryErr
		}
		defer rows.Close()

		for rows.Next() {
			value, scanErr := scanPosition(rows)
			if scanErr != nil {
				return scanErr
			}
			result = append(result, value)
		}

		return rows.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	return result, nil
}
