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
)

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

	appCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	cont, err := container.New(appCtx, cfg)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	serveDone := make(chan struct{})
	var serveErr error
	go func() {
		defer close(serveDone)
		serveErr = cont.HTTPServer.ListenAndServe()
	}()
	cont.Logger.Info("HTTP server is starting", "address", cfg.HTTP.Addr)

	cleanupContext, cancelCleanup := context.WithCancel(appCtx)
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		cont.CleanupJob.Run(cleanupContext)
	}()
	defer func() {
		cancelCleanup()
		<-cleanupDone
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
	<-cleanupDone

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
