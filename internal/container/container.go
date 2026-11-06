package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/config"
	httpapi "github.com/MaksimKosarevHSE/avito-lab/internal/http"
	createhttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/create"
	gethttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/get"
	"github.com/go-chi/chi/v5"

	httpmiddleware "github.com/MaksimKosarevHSE/avito-lab/internal/http/middleware"
	finishhttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/finish"
	"github.com/MaksimKosarevHSE/avito-lab/internal/idempotency"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres/transaction"
	"github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/finish"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/get"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	serviceName         = "trip-service"
	maxRequestBodyBytes = 1 << 20
)

type Container struct {
	HTTPServer *http.Server
	CleanupJob *idempotency.CleanupJob
	Logger     *slog.Logger

	pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {

	openAPISpec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load embedded OpenAPI specification: %w", err)
	}
	if err := openAPISpec.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate embedded OpenAPI specification: %w", err)
	}
	requestValidator := httpmiddleware.OpenAPIValidator(openAPISpec)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})).With("service", serviceName)
	slog.SetDefault(logger)

	pool, err := postgres.Open(ctx, postgres.PoolConfig{
		URL:             cfg.Database.URL,
		MaxConns:        cfg.Database.MaxConns,
		MinConns:        cfg.Database.MinConns,
		MaxConnLifetime: cfg.Database.MaxConnLifetime,
		ConnectTimeout:  cfg.Database.ConnectTimeout,
	})

	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}

	containerIsCreated := false
	defer func() {
		if !containerIsCreated {
			pool.Close()
		}
	}()

	txManager, err := transaction.New(pool, cfg.Database.QueryTimeout)
	if err != nil {
		return nil, fmt.Errorf("create transaction manager: %w", err)
	}

	database, err := postgres.NewDB(pool, txManager, cfg.Database.QueryTimeout)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL gateway: %w", err)
	}

	repository := trip.New(database)
	createService := create.NewService(txManager, repository, cfg.IdempotencyTTL)
	getService := get.NewService(repository)
	finishService := finish.NewService(txManager, repository)
	httpHandler := httpapi.NewHandler(
		createhttp.New(createService),
		gethttp.New(getService),
		finishhttp.New(finishService),
		pool,
		cfg.Database.QueryTimeout,
	)

	router := chi.NewRouter()
	router.Use(httpmiddleware.BodyLimit(maxRequestBodyBytes))
	server := &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: api.HandlerWithOptions(httpHandler, api.ChiServerOptions{
			BaseRouter:       router,
			Middlewares:      []api.MiddlewareFunc{requestValidator},
			ErrorHandlerFunc: httpHandler.HandleParameterError,
		}),
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	cleanupJob := idempotency.NewCleanupJob(repository,
		cfg.IdempotencyCleanupInterval,
		logger,
	)

	containerIsCreated = true

	return &Container{
		HTTPServer: server,
		CleanupJob: cleanupJob,
		Logger:     logger,
		pool:       pool,
	}, nil
}

func (c *Container) Close(ctx context.Context) error {
	if c == nil || c.pool == nil {
		return nil
	}

	return postgres.Close(ctx, c.pool)
}
