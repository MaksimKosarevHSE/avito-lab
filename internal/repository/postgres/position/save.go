package position

import (
	"context"
	"fmt"
	"strings"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

func (r *Repository) Create(ctx context.Context, pos position.Position) (position.Position, error) {
	query, args, err := r.builder.
		Insert(positionsTable).
		Columns(positionInsertColumns...).
		Values(
			pos.TripID,
			pos.Latitude,
			pos.Longitude,
			pos.RecordedAt,
			pos.CreatedAt,
		).
		Suffix("RETURNING " + strings.Join(positionColumns, ", ")).
		ToSql()
	if err != nil {
		return position.Position{}, fmt.Errorf("build create position query: %w", err)
	}

	var created position.Position

	err = r.db.Execute(
		ctx,
		func(queryCtx context.Context, executor postgres.Executor) error {
			var scanErr error

			created, scanErr = scanPosition(
				executor.QueryRow(queryCtx, query, args...),
			)

			return scanErr
		},
	)
	if err != nil {
		return position.Position{},
			fmt.Errorf("create position: %w", err)
	}

	return created, nil
}
