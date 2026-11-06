package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The pool is part of the key so transactions opened for different databases
// cannot be mixed when they share the same context.
type transactionContextKey struct {
	pool *pgxpool.Pool
}

// Manager owns transaction boundaries and the transaction stored in context.
type Manager struct {
	pool           *pgxpool.Pool
	controlTimeout time.Duration
}

func New(pool *pgxpool.Pool, controlTimeout time.Duration) (*Manager, error) {
	if pool == nil {
		return nil, errors.New("transaction manager: pool is nil")
	}
	if controlTimeout <= 0 {
		return nil, errors.New("transaction manager: control timeout must be positive")
	}

	return &Manager{
		pool:           pool,
		controlTimeout: controlTimeout,
	}, nil
}

// Do executes fn inside a transaction. A nested call joins the current
// transaction; it does not create a savepoint.
func (m *Manager) Do(
	ctx context.Context,
	fn func(ctx context.Context) error,
) (err error) {
	if fn == nil {
		return errors.New("transaction manager: transaction function is nil")
	}

	if _, ok := m.Transaction(ctx); ok {
		return fn(ctx)
	}

	beginCtx, cancelBegin := m.controlContext(ctx)
	tx, beginErr := m.pool.BeginTx(beginCtx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	cancelBegin()
	if beginErr != nil {
		return fmt.Errorf("begin transaction: %w", beginErr)
	}

	txCtx := context.WithValue(
		ctx,
		transactionContextKey{pool: m.pool},
		tx,
	)
	committed := false
	defer func() {
		if committed {
			return
		}

		if rollbackErr := m.rollback(txCtx, tx); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	if err = fn(txCtx); err != nil {
		return err
	}

	commitCtx, cancelCommit := m.controlContext(ctx)
	commitErr := tx.Commit(commitCtx)
	cancelCommit()
	if commitErr != nil {
		return fmt.Errorf("commit transaction: %w", commitErr)
	}

	committed = true
	return nil
}

// Transaction returns the transaction associated with ctx for this manager's
// pool. It is intended for the PostgreSQL gateway, not for business code.
func (m *Manager) Transaction(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(transactionContextKey{pool: m.pool}).(pgx.Tx)
	return tx, ok
}

func (m *Manager) controlContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, m.controlTimeout)
}

func (m *Manager) rollback(ctx context.Context, tx pgx.Tx) error {
	rollbackCtx, cancelRollback := context.WithTimeout(
		context.WithoutCancel(ctx),
		m.controlTimeout,
	)
	defer cancelRollback()

	if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback transaction: %w", err)
	}

	return nil
}
