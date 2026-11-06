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
	QueryTimeout    time.Duration
}

type Pool struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func Open(ctx context.Context, cfg PoolConfig) (*Pool, error) {
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

	pool := &Pool{
		pool:         pgxPool,
		queryTimeout: cfg.QueryTimeout,
	}

	pingCtx, cancelPing := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancelPing()
	if err := pgxPool.Ping(pingCtx); err != nil {
		pgxPool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return pool, nil
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
	case cfg.QueryTimeout <= 0:
		return errors.New("PostgreSQL query timeout must be positive")
	default:
		return nil
	}
}

func (p *Pool) Ping(ctx context.Context) error {
	queryCtx, cancel := p.queryContext(ctx)
	defer cancel()

	if err := p.pool.Ping(queryCtx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return nil
}

func (p *Pool) Close() {
	p.pool.Close()
}

func (p *Pool) queryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, p.queryTimeout)
}
