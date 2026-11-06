package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/MaksimKosarevHSE/avito-lab/internal/trip"
	"github.com/jackc/pgx/v5"
)

type txManager struct {
	pool *Pool
}

func NewTxManager(pool *Pool) trip.TxManager {
	return &txManager{pool: pool}
}

func (m *txManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	beginCtx, cancelBegin := m.pool.queryContext(ctx)
	tx, err := m.pool.pool.BeginTx(beginCtx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	cancelBegin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, transactionContextKey{}, tx)
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = m.rollback(txCtx, tx)
			panic(recovered)
		}
	}()

	if err := fn(txCtx); err != nil {
		if rollbackErr := m.rollback(txCtx, tx); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}

		return err
	}

	commitCtx, cancelCommit := m.pool.queryContext(ctx)
	defer cancelCommit()
	if err := tx.Commit(commitCtx); err != nil {
		commitErr := fmt.Errorf("commit transaction: %w", err)
		if rollbackErr := m.rollback(txCtx, tx); rollbackErr != nil {
			return errors.Join(commitErr, rollbackErr)
		}

		return commitErr
	}

	return nil
}

func (m *txManager) rollback(ctx context.Context, tx pgx.Tx) error {
	rollbackCtx, cancelRollback := context.WithTimeout(
		context.WithoutCancel(ctx),
		m.pool.queryTimeout,
	)
	defer cancelRollback()

	if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback transaction: %w", err)
	}

	return nil
}
