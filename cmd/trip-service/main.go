package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/config"
	"github.com/MaksimKosarevHSE/avito-lab/internal/httpapi"
	"github.com/MaksimKosarevHSE/avito-lab/internal/postgres"
	"github.com/MaksimKosarevHSE/avito-lab/internal/trip"
	"github.com/go-chi/chi/v5"
)

const serviceName = "trip-service"

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})).With("service", serviceName)
	slog.SetDefault(logger)

	appCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	database, err := postgres.Open(appCtx, postgres.PoolConfig{
		URL:             cfg.Database.URL,
		MaxConns:        cfg.Database.MaxConns,
		MinConns:        cfg.Database.MinConns,
		MaxConnLifetime: cfg.Database.MaxConnLifetime,
		ConnectTimeout:  cfg.Database.ConnectTimeout,
		QueryTimeout:    cfg.Database.QueryTimeout,
	})
	if err != nil {
		return fmt.Errorf("open PostgreSQL: %w", err)
	}
	databaseCloseStarted := false
	defer func() {
		if !databaseCloseStarted {
			database.Close()
		}
	}()

	repository := postgres.NewTripRepository(database)
	txManager := postgres.NewTxManager(database)
	tripService := trip.NewService(txManager, repository, cfg.IdempotencyTTL)
	httpHandler := httpapi.NewHandler(tripService, database, cfg.Database.QueryTimeout)

	router := chi.NewRouter()
	server := &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: api.HandlerWithOptions(httpHandler, api.ChiServerOptions{
			BaseRouter:       router,
			ErrorHandlerFunc: httpHandler.HandleParameterError,
		}),
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	logger.Info("HTTP server is starting", "address", cfg.HTTP.Addr)

	select {
	case serveErr := <-serverErrors:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("serve HTTP: %w", serveErr)
	case <-appCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		cfg.ShutdownTimeout,
	)
	defer cancelShutdown()

	var shutdownErr error
	if err := server.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("graceful HTTP shutdown: %w", err)
		if closeErr := server.Close(); closeErr != nil {
			shutdownErr = errors.Join(
				shutdownErr,
				fmt.Errorf("force close HTTP server: %w", closeErr),
			)
		}
	} else {
		if serveErr := <-serverErrors; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			shutdownErr = fmt.Errorf("serve HTTP during shutdown: %w", serveErr)
		}
	}

	databaseCloseStarted = true
	if err := closeDatabase(shutdownCtx, database); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	if shutdownErr != nil {
		return shutdownErr
	}

	logger.Info("service stopped gracefully")
	return nil
}

func closeDatabase(ctx context.Context, database *postgres.Pool) error {
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
