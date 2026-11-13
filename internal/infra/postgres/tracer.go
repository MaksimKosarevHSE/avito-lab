package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	dbInstrumentationName = "github.com/MaksimKosarevHSE/avito-lab/internal/infra/postgres"
	unknownCollection     = "unknown"
)

type queryObservationKey struct{}

type queryObservation struct {
	startedAt  time.Time
	operation  string
	collection string
}

type DBTracer struct {
	*otelpgx.Tracer

	duration metric.Float64Histogram
}

var (
	_ pgx.QueryTracer       = (*DBTracer)(nil)
	_ pgx.BatchTracer       = (*DBTracer)(nil)
	_ pgx.CopyFromTracer    = (*DBTracer)(nil)
	_ pgx.PrepareTracer     = (*DBTracer)(nil)
	_ pgx.ConnectTracer     = (*DBTracer)(nil)
	_ pgxpool.AcquireTracer = (*DBTracer)(nil)
)

func NewDBTracer(
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
) (*DBTracer, error) {
	if tracerProvider == nil {
		return nil, errors.New("PostgreSQL tracer provider is nil")
	}
	if meterProvider == nil {
		return nil, errors.New("PostgreSQL meter provider is nil")
	}

	duration, err := meterProvider.Meter(dbInstrumentationName).Float64Histogram(
		"db.client.operation.duration",
		metric.WithDescription("Duration of PostgreSQL client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001,
			0.005,
			0.01,
			0.05,
			0.1,
			0.5,
			1,
			5,
			10,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL duration histogram: %w", err)
	}

	base := otelpgx.NewTracer(
		otelpgx.WithTracerProvider(tracerProvider),
		otelpgx.WithMeterProvider(metricnoop.NewMeterProvider()),
	)

	return &DBTracer{
		Tracer:   base,
		duration: duration,
	}, nil
}

func (t *DBTracer) TraceQueryStart(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)

	observation := queryObservation{
		startedAt:  time.Now(),
		operation:  operationFromSQL(ctx, data.SQL),
		collection: collectionFromSQL(data.SQL),
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(semconv.DBCollectionName(observation.collection))
	}

	return context.WithValue(ctx, queryObservationKey{}, observation)
}

func (t *DBTracer) TraceQueryEnd(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryEndData,
) {
	if observation, ok := ctx.Value(queryObservationKey{}).(queryObservation); ok {
		attrs := []attribute.KeyValue{
			semconv.DBOperationName(observation.operation),
			semconv.DBCollectionName(observation.collection),
		}
		if data.Err != nil {
			errorType := boundedErrorType(data.Err)
			attrs = append(attrs, semconv.ErrorTypeKey.String(errorType))

			span := trace.SpanFromContext(ctx)
			if span.IsRecording() {
				span.SetAttributes(semconv.ErrorTypeKey.String(errorType))
				span.SetStatus(codes.Error, errorType)
			}
		}

		t.duration.Record(
			ctx,
			time.Since(observation.startedAt).Seconds(),
			metric.WithAttributes(attrs...),
		)
	}

	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

func operationFromSQL(ctx context.Context, statement string) string {
	operation := otelpgx.SQLOperationName(ctx, statement)
	if operation != "WITH" {
		return operation
	}

	fields := strings.Fields(statement)
	for _, candidate := range []string{"DELETE", "INSERT", "UPDATE", "MERGE"} {
		for _, field := range fields {
			if strings.ToUpper(strings.Trim(field, "(),;")) == candidate {
				return candidate
			}
		}
	}

	return "SELECT"
}

func collectionFromSQL(statement string) string {
	fields := strings.Fields(statement)
	for index, field := range fields {
		switch strings.ToUpper(strings.Trim(field, "(),;")) {
		case "FROM", "INTO", "UPDATE", "JOIN":
			if index+1 >= len(fields) {
				continue
			}

			candidate := normalizeCollection(fields[index+1])
			switch candidate {
			case "trips", "trip_status_history", "trip_create_idempotency":
				return candidate
			}
		}
	}

	return unknownCollection
}

func normalizeCollection(value string) string {
	value = strings.Trim(value, "\"`()[],;")
	value = strings.ReplaceAll(value, "\"", "")
	if separator := strings.LastIndexByte(value, '.'); separator >= 0 {
		value = value[separator+1:]
	}

	return strings.ToLower(strings.Trim(value, "\"`()[],;"))
}

func boundedErrorType(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "context.deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "context.canceled"
	case errors.Is(err, pgx.ErrNoRows):
		return "not_found"
	}

	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		return "postgresql." + postgresError.Code
	}

	return "other"
}
