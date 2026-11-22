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

	"github.com/MaksimKosarevHSE/avito-lab/internal/config"
	"github.com/MaksimKosarevHSE/avito-lab/internal/container"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/telemetry"
)

var version = "dev"

func main() {
	logger := telemetry.NewLogger(os.Stdout, slog.LevelInfo, "trip-service")
	if err := run(); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	appCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	cont, err := container.New(appCtx, cfg, version)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	serveDone := make(chan struct{})
	var serveErr error
	go func() {
		defer close(serveDone)
		serveErr = cont.HTTPServer.ListenAndServe()
	}()
	cont.Logger.Info("service started", "version", version)
	cont.Logger.Info("HTTP server is starting", "address", cfg.HTTP.Addr)

	cleanupContext, cancelCleanup := context.WithCancel(appCtx)
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		cont.CleanupJob.Run(cleanupContext)
	}()

	selectionContext, cancelSelection := context.WithCancel(context.Background())
	selectionDone := make(chan struct{})
	go func() {
		defer close(selectionDone)
		cont.TripSelectionJob.Run(selectionContext)
	}()

	workerContext, cancelWorkers := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		cont.PushWorkerPool.Start(workerContext)
	}()

	var shutdownErr error
	select {
	case <-serveDone:
		// Сервер завершился сам
	case <-appCtx.Done():
		cont.Logger.Info("shutdown signal received")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		cfg.ShutdownTimeout,
	)
	defer cancelShutdown()

	if err := cont.HTTPServer.Shutdown(shutdownCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("graceful HTTP shutdown: %w", err))
		if closeErr := cont.HTTPServer.Close(); closeErr != nil {
			shutdownErr = errors.Join(
				shutdownErr,
				fmt.Errorf("force close HTTP server: %w", closeErr),
			)
		}
	}

	cancelCleanup()
	cancelSelection()
	select {
	case <-cleanupDone:
	case <-shutdownCtx.Done():
		shutdownErr = errors.Join(
			shutdownErr,
			fmt.Errorf("stop idempotency cleanup: %w", shutdownCtx.Err()),
		)
	}

	select {
	case <-selectionDone:
	case <-shutdownCtx.Done():
		shutdownErr = errors.Join(
			shutdownErr,
			fmt.Errorf("stop trip selection job: %w", shutdownCtx.Err()),
		)
	}

	cont.PushWorkerPool.Stop()
	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
		shutdownErr = errors.Join(
			shutdownErr,
			fmt.Errorf("stop push workers: %w", shutdownCtx.Err()),
		)
		cancelWorkers()
		<-workerDone
	}
	cancelWorkers()

	<-serveDone
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve HTTP: %w", serveErr))
	}

	if err := cont.Close(shutdownCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if shutdownErr != nil {
		return fmt.Errorf("service shutdown: %w", shutdownErr)
	}

	cont.Logger.Info("service stopped gracefully")
	return nil
}
