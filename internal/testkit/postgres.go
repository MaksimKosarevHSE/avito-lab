//go:build integration

package testkit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	postgresinfra "github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres/transaction"
	migrationsfs "github.com/MaksimKosarevHSE/avito-lab/migrations"
)

const (
	postgresImage   = "postgres:16-alpine"
	databaseName    = "trip_test"
	databaseUser    = "trip_test"
	databasePass    = "trip_test"
	databaseTimeout = 3 * time.Second
)

type Postgres struct {
	Pool      *pgxpool.Pool
	Database  *postgresinfra.DB
	TxManager *transaction.Manager
}

func NewPostgres(t testing.TB) *Postgres {
	t.Helper()

	ctx := t.Context()
	container, err := tcpostgres.Run(
		ctx,
		postgresImage,
		tcpostgres.WithDatabase(databaseName),
		tcpostgres.WithUsername(databaseUser),
		tcpostgres.WithPassword(databasePass),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, applyMigrations(ctx, databaseURL))

	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	txManager, err := transaction.New(pool, databaseTimeout)
	require.NoError(t, err)

	database, err := postgresinfra.NewDB(pool, txManager, databaseTimeout)
	require.NoError(t, err)

	return &Postgres{
		Pool:      pool,
		Database:  database,
		TxManager: txManager,
	}
}

func (p *Postgres) ResetTrips(t testing.TB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), databaseTimeout)
	defer cancel()

	_, err := p.Pool.Exec(
		ctx,
		`TRUNCATE TABLE
			trip_create_idempotency,
			trip_status_history,
			trips
		 RESTART IDENTITY CASCADE`,
	)
	require.NoError(t, err)
}

func applyMigrations(ctx context.Context, databaseURL string) (err error) {
	migrationDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer func() {
		if closeErr := migrationDB.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close migration database: %w", closeErr))
		}
	}()

	if err := migrationDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		migrationDB,
		migrationsfs.FS,
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
