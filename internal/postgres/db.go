package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type dbtx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type transactionContextKey struct{}

func executorFromContext(ctx context.Context, pool *Pool) dbtx {
	if tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return tx
	}

	return pool.pool
}
