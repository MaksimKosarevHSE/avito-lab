package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type transactionProvider interface {
	Transaction(ctx context.Context) (pgx.Tx, bool)
}

type DB struct {
	pool         *pgxpool.Pool
	transactions transactionProvider
	queryTimeout time.Duration
}

func NewDB(
	pool *pgxpool.Pool,
	transactions transactionProvider,
	queryTimeout time.Duration,
) (*DB, error) {
	if pool == nil {
		return nil, errors.New("postgres DB: pool is nil")
	}
	if transactions == nil {
		return nil, errors.New("postgres DB: transaction provider is nil")
	}
	if queryTimeout <= 0 {
		return nil, errors.New("postgres DB: query timeout must be positive")
	}

	return &DB{
		pool:         pool,
		transactions: transactions,
		queryTimeout: queryTimeout,
	}, nil
}

func (db *DB) Execute(
	ctx context.Context,
	fn func(queryCtx context.Context, executor Executor) error,
) error {
	if fn == nil {
		return errors.New("postgres DB: query function is nil")
	}

	queryCtx, cancel := context.WithTimeout(ctx, db.queryTimeout)
	defer cancel()

	return fn(queryCtx, db.executor(ctx))
}

func (db *DB) executor(ctx context.Context) Executor {
	if tx, ok := db.transactions.Transaction(ctx); ok {
		return tx
	}

	return db.pool
}
