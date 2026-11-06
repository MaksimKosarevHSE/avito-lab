package trip

import (
	"context"
	"fmt"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

func (r *Repository) AddStatusHistory(
	ctx context.Context,
	history domaintrip.StatusHistory,
) error {
	var fromStatus any
	if history.FromStatus != nil {
		fromStatus = string(*history.FromStatus)
	}

	var reason any
	if history.Reason != nil {
		reason = *history.Reason
	}

	query, args, err := r.builder.
		Insert(tripStatusHistoryTable).
		Columns(statusHistoryColumns...).
		Values(
			history.TripID,
			fromStatus,
			string(history.ToStatus),
			reason,
			history.ChangedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build add trip status history query: %w", err)
	}

	err = r.db.Execute(ctx, func(queryCtx context.Context, executor postgres.Executor) error {
		_, execErr := executor.Exec(queryCtx, query, args...)
		return execErr
	})
	if err != nil {
		return fmt.Errorf("add trip status history: %w", err)
	}

	return nil
}
