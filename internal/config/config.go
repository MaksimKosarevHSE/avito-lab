package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	httpAddrEnv              = "HTTP_ADDR"
	httpReadTimeoutEnv       = "HTTP_READ_TIMEOUT"
	httpReadHeaderTimeoutEnv = "HTTP_READ_HEADER_TIMEOUT"
	httpWriteTimeoutEnv      = "HTTP_WRITE_TIMEOUT"
	httpIdleTimeoutEnv       = "HTTP_IDLE_TIMEOUT"
	logLevelEnv              = "LOG_LEVEL"
	shutdownTimeoutEnv       = "SHUTDOWN_TIMEOUT"
	idempotencyTTLEnv        = "IDEMPOTENCY_TTL"
	databaseURLEnv           = "DATABASE_URL"
	databaseMaxConnsEnv      = "DATABASE_MAX_CONNS"
	databaseMinConnsEnv      = "DATABASE_MIN_CONNS"
	databaseMaxLifetimeEnv   = "DATABASE_MAX_CONN_LIFETIME"
	databaseConnectEnv       = "DATABASE_CONNECT_TIMEOUT"
	databaseQueryEnv         = "DATABASE_QUERY_TIMEOUT"
)

type Config struct {
	HTTP            HTTPConfig
	Database        DatabaseConfig
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	IdempotencyTTL  time.Duration
}

type HTTPConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func Load() (Config, error) {
	var (
		cfg Config
		err error
	)

	cfg.HTTP.Addr, err = requiredString(httpAddrEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTP.ReadTimeout, err = requiredPositiveDuration(httpReadTimeoutEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTP.ReadHeaderTimeout, err = requiredPositiveDuration(httpReadHeaderTimeoutEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTP.WriteTimeout, err = requiredPositiveDuration(httpWriteTimeoutEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTP.IdleTimeout, err = requiredPositiveDuration(httpIdleTimeoutEnv)
	if err != nil {
		return Config{}, err
	}

	logLevel, err := requiredString(logLevelEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel, err = parseLogLevel(logLevel)
	if err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout, err = requiredPositiveDuration(shutdownTimeoutEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.IdempotencyTTL, err = requiredPositiveDuration(idempotencyTTLEnv)
	if err != nil {
		return Config{}, err
	}
	if cfg.IdempotencyTTL < time.Microsecond {
		return Config{}, fmt.Errorf("%s must be at least %s", idempotencyTTLEnv, time.Microsecond)
	}

	cfg.Database.URL, err = requiredString(databaseURLEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.Database.MaxConns, err = requiredInt32(databaseMaxConnsEnv, 1)
	if err != nil {
		return Config{}, err
	}
	cfg.Database.MinConns, err = requiredInt32(databaseMinConnsEnv, 0)
	if err != nil {
		return Config{}, err
	}
	if cfg.Database.MinConns > cfg.Database.MaxConns {
		return Config{}, fmt.Errorf(
			"%s must be less than or equal to %s",
			databaseMinConnsEnv,
			databaseMaxConnsEnv,
		)
	}
	cfg.Database.MaxConnLifetime, err = requiredPositiveDuration(databaseMaxLifetimeEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.Database.ConnectTimeout, err = requiredPositiveDuration(databaseConnectEnv)
	if err != nil {
		return Config{}, err
	}
	cfg.Database.QueryTimeout, err = requiredPositiveDuration(databaseQueryEnv)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func requiredString(name string) (string, error) {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}

	return strings.TrimSpace(value), nil
}

func requiredPositiveDuration(name string) (time.Duration, error) {
	value, err := requiredString(name)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}

	return duration, nil
}

func requiredInt32(name string, minimum int64) (int32, error) {
	value, err := requiredString(name)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid 32-bit integer: %w", name, err)
	}
	if parsed < minimum {
		return 0, fmt.Errorf("%s must be greater than or equal to %d", name, minimum)
	}

	return int32(parsed), nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%s must be one of: debug, info, warn, error", logLevelEnv)
	}
}
