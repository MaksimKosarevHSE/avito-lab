package trip

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	query, args, err := r.builder.
		Select("count(*)").
		From(tripsTable).
		Where(sq.Eq{statusColumn: string(domaintrip.StatusActive)}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count active trips query: %w", err)
	}

	var count int64
	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		return executor.QueryRow(queryCtx, query, args...).Scan(&count)
	})
	if err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}

	return count, nil
}
