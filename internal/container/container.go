package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/codes"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/config"
	httpapi "github.com/MaksimKosarevHSE/avito-lab/internal/http"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
	httpmiddleware "github.com/MaksimKosarevHSE/avito-lab/internal/http/middleware"
	createhttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/create"
	finishhttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/finish"
	gethttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/get"
	"github.com/MaksimKosarevHSE/avito-lab/internal/idempotency"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres/transaction"
	"github.com/MaksimKosarevHSE/avito-lab/internal/infra/telemetry"
	"github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/finish"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/get"
)

const (
	serviceName         = "trip-service"
	maxRequestBodyBytes = 1 << 20
)

type Container struct {
	HTTPServer *http.Server
	CleanupJob *idempotency.CleanupJob
	Logger     *slog.Logger

	pool      *pgxpool.Pool
	telemetry *telemetry.Providers
}

func New(
	ctx context.Context,
	cfg config.Config,
	serviceVersion string,
) (_ *Container, resultErr error) {
	openAPISpec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load embedded OpenAPI specification: %w", err)
	}
	if err := openAPISpec.Validate(ctx); err != nil {
		return nil, fmt.Errorf("validate embedded OpenAPI specification: %w", err)
	}
	logger := telemetry.NewLogger(os.Stdout, cfg.LogLevel, serviceName)
	providers, err := telemetry.New(ctx, logger, serviceName, serviceVersion)
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry: %w", err)
	}

	var (
		pool        *pgxpool.Pool
		tripMetrics *telemetry.TripMetrics
		created     bool
	)
	defer func() {
		if created {
			return
		}

		cleanupCtx, cancelCleanup := context.WithTimeout(
			context.WithoutCancel(ctx),
			cfg.ShutdownTimeout,
		)
		defer cancelCleanup()

		if pool != nil {
			resultErr = errors.Join(resultErr, postgres.Close(cleanupCtx, pool))
		}
		resultErr = errors.Join(resultErr, providers.Shutdown(cleanupCtx))
	}()

	startupCtx, startupSpan := providers.Traces.
		Tracer(serviceName+"/startup").
		Start(ctx, "service initialization")
	defer func() {
		if resultErr != nil {
			startupSpan.RecordError(resultErr)
			startupSpan.SetStatus(codes.Error, "service initialization failed")
		}
		startupSpan.End()
	}()

	dbTracer, err := postgres.NewDBTracer(providers.Traces, providers.Metrics)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL telemetry: %w", err)
	}

	pool, err = postgres.Open(startupCtx, postgres.PoolConfig{
		URL:             cfg.Database.URL,
		MaxConns:        cfg.Database.MaxConns,
		MinConns:        cfg.Database.MinConns,
		MaxConnLifetime: cfg.Database.MaxConnLifetime,
		ConnectTimeout:  cfg.Database.ConnectTimeout,
		Tracer:          dbTracer,
	})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}

	txManager, err := transaction.New(pool, cfg.Database.QueryTimeout)
	if err != nil {
		return nil, fmt.Errorf("create transaction manager: %w", err)
	}

	database, err := postgres.NewDB(pool, txManager, cfg.Database.QueryTimeout)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL gateway: %w", err)
	}

	repository := trip.New(database)
	activeTrips, err := repository.CountActive(startupCtx)
	if err != nil {
		return nil, fmt.Errorf("initialize active trips metric: %w", err)
	}

	tripMetrics, err = telemetry.NewTripMetrics(
		providers.Metrics.Meter(serviceName+"/business"),
		activeTrips,
	)
	if err != nil {
		return nil, fmt.Errorf("create trip metrics: %w", err)
	}
	httpMetrics, err := httpmiddleware.NewHTTPMetrics(
		providers.Metrics.Meter(serviceName + "/http"),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP metrics: %w", err)
	}

	createService := create.NewService(
		txManager,
		repository,
		cfg.IdempotencyTTL,
		create.WithObservability(logger, tripMetrics),
	)
	getService := get.NewService(repository)
	finishService := finish.NewService(
		txManager,
		repository,
		finish.WithObservability(logger, tripMetrics),
	)
	httpHandler := httpapi.NewHandler(
		createhttp.New(createService, logger),
		gethttp.New(getService, logger),
		finishhttp.New(finishService, logger),
		pool,
		cfg.Database.QueryTimeout,
		logger,
	)

	problems := domainerror.NewWriter(logger)
	requestValidator := httpmiddleware.OpenAPIValidator(openAPISpec, problems)
	router := chi.NewRouter()
	router.Use(httpmiddleware.BodyLimit(maxRequestBodyBytes, problems))
	generatedHandler := api.HandlerWithOptions(httpHandler, api.ChiServerOptions{
		BaseRouter:       router,
		Middlewares:      []api.MiddlewareFunc{requestValidator},
		ErrorHandlerFunc: httpHandler.HandleParameterError,
	})
	observedHandler := httpmiddleware.Recover(logger)(
		httpmiddleware.Tracing(providers.Traces, router)(
			httpmiddleware.Logging(logger, router)(
				httpmiddleware.Metrics(httpMetrics, router)(generatedHandler),
			),
		),
	)
	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           observedHandler,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	cleanupJob := idempotency.NewCleanupJob(repository,
		cfg.IdempotencyCleanupInterval,
		logger,
		providers.Traces.Tracer(serviceName+"/idempotency-cleanup"),
	)

	created = true

	return &Container{
		HTTPServer: server,
		CleanupJob: cleanupJob,
		Logger:     logger,
		pool:       pool,
		telemetry:  providers,
	}, nil
}

func (c *Container) Close(ctx context.Context) error {
	if c == nil {
		return nil
	}

	var result error
	if c.pool != nil {
		result = errors.Join(result, postgres.Close(ctx, c.pool))
	}
	if c.telemetry != nil {
		if err := c.telemetry.Shutdown(ctx); err != nil {
			c.Logger.WarnContext(
				context.WithoutCancel(ctx),
				"OpenTelemetry shutdown failed",
				"error", err,
			)
		}
	}

	return result
}
