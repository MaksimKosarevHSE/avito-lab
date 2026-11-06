package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
}

func Open(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	if err := validatePoolConfig(cfg); err != nil {
		return nil, err
	}

	pgxConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL pool configuration: %w", err)
	}

	pgxConfig.MaxConns = cfg.MaxConns
	pgxConfig.MinConns = cfg.MinConns
	pgxConfig.MaxConnLifetime = cfg.MaxConnLifetime
	pgxConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pgxPool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	pingCtx, cancelPing := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancelPing()
	if err := pgxPool.Ping(pingCtx); err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return pgxPool, nil
}

func Close(ctx context.Context, database *pgxpool.Pool) error {
	closed := make(chan struct{})

	go func() {
		database.Close()
		close(closed)
	}()

	select {
	case <-closed:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("close PostgreSQL pool: %w", ctx.Err())
	}
}

func validatePoolConfig(cfg PoolConfig) error {
	switch {
	case cfg.URL == "":
		return errors.New("PostgreSQL URL is required")
	case cfg.MaxConns <= 0:
		return errors.New("PostgreSQL max connections must be positive")
	case cfg.MinConns < 0:
		return errors.New("PostgreSQL min connections must not be negative")
	case cfg.MinConns > cfg.MaxConns:
		return errors.New("PostgreSQL min connections must not exceed max connections")
	case cfg.MaxConnLifetime <= 0:
		return errors.New("PostgreSQL max connection lifetime must be positive")
	case cfg.ConnectTimeout <= 0:
		return errors.New("PostgreSQL connect timeout must be positive")
	default:
		return nil
	}
}
