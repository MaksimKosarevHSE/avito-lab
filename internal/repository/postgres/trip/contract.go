package trip

import (
	"context"

	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
)

type database interface {
	Execute(
		ctx context.Context,
		fn func(queryCtx context.Context, executor postgres.Executor) error,
	) error
}

var _ database = (*postgres.DB)(nil)
