package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func TestCollectionFromSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		statement string
		want      string
	}{
		{
			name:      "select trips",
			statement: "SELECT id FROM trips WHERE id = $1",
			want:      "trips",
		},
		{
			name:      "insert history",
			statement: "INSERT INTO trip_status_history (trip_id) VALUES ($1)",
			want:      "trip_status_history",
		},
		{
			name:      "update schema qualified idempotency",
			statement: `UPDATE "public"."trip_create_idempotency" SET expires_at = $1`,
			want:      "trip_create_idempotency",
		},
		{
			name:      "unknown",
			statement: "SELECT 1",
			want:      unknownCollection,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, collectionFromSQL(test.statement))
		})
	}
}

func TestBoundedErrorType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "deadline",
			err:  fmt.Errorf("query: %w", context.DeadlineExceeded),
			want: "context.deadline_exceeded",
		},
		{
			name: "canceled",
			err:  context.Canceled,
			want: "context.canceled",
		},
		{
			name: "not found",
			err:  pgx.ErrNoRows,
			want: "not_found",
		},
		{
			name: "postgres SQLSTATE",
			err:  &pgconn.PgError{Code: "23505"},
			want: "postgresql.23505",
		},
		{
			name: "other",
			err:  errors.New("failure with changing details"),
			want: "other",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, boundedErrorType(test.err))
		})
	}
}

func TestOperationFromSQLResolvesDataChangingCTE(t *testing.T) {
	statement := `WITH deleted AS (
		DELETE FROM trip_create_idempotency WHERE expires_at < $1 RETURNING 1
	) SELECT count(*) FROM deleted`

	assert.Equal(t, "DELETE", operationFromSQL(context.Background(), statement))
}

func TestDBTracerRecordsBoundedMetricAndSpanAttributes(t *testing.T) {
	spanExporter := tracetest.NewInMemoryExporter()
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(spanExporter),
	)
	t.Cleanup(func() {
		require.NoError(t, traceProvider.Shutdown(context.Background()))
	})

	metricReader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	t.Cleanup(func() {
		require.NoError(t, meterProvider.Shutdown(context.Background()))
	})

	tracer, err := NewDBTracer(traceProvider, meterProvider)
	require.NoError(t, err)

	queryErr := &pgconn.PgError{Code: "23505"}
	ctx, parentSpan := traceProvider.Tracer("test").Start(context.Background(), "request")
	defer parentSpan.End()

	ctx = tracer.TraceQueryStart(
		ctx,
		nil,
		pgx.TraceQueryStartData{SQL: "INSERT INTO trips (id) VALUES ($1)"},
	)
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: queryErr})

	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assertAttribute(t, spans[0].Attributes, semconv.DBCollectionNameKey, "trips")
	assertAttribute(t, spans[0].Attributes, semconv.ErrorTypeKey, "postgresql.23505")

	var resourceMetrics metricdata.ResourceMetrics
	require.NoError(t, metricReader.Collect(context.Background(), &resourceMetrics))

	histogram := findDurationHistogram(t, resourceMetrics)
	require.Len(t, histogram.DataPoints, 1)
	assert.EqualValues(t, 1, histogram.DataPoints[0].Count)
	attributes := histogram.DataPoints[0].Attributes
	assertSetAttribute(t, attributes, semconv.DBOperationNameKey, "INSERT")
	assertSetAttribute(t, attributes, semconv.DBCollectionNameKey, "trips")
	assertSetAttribute(t, attributes, semconv.ErrorTypeKey, "postgresql.23505")
}

func TestDBTracerMarksNoRowsSpanAsError(t *testing.T) {
	spanExporter := tracetest.NewInMemoryExporter()
	traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExporter))
	t.Cleanup(func() { require.NoError(t, traceProvider.Shutdown(context.Background())) })

	meterProvider := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { require.NoError(t, meterProvider.Shutdown(context.Background())) })

	tracer, err := NewDBTracer(traceProvider, meterProvider)
	require.NoError(t, err)
	ctx, parentSpan := traceProvider.Tracer("test").Start(context.Background(), "request")
	defer parentSpan.End()

	ctx = tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT id FROM trips"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: pgx.ErrNoRows})

	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assertAttribute(t, spans[0].Attributes, semconv.ErrorTypeKey, "not_found")
}

func findDurationHistogram(
	t *testing.T,
	resourceMetrics metricdata.ResourceMetrics,
) metricdata.Histogram[float64] {
	t.Helper()

	for _, scope := range resourceMetrics.ScopeMetrics {
		for _, current := range scope.Metrics {
			if current.Name != "db.client.operation.duration" {
				continue
			}

			histogram, ok := current.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			assert.Equal(t, "s", current.Unit)
			return histogram
		}
	}

	require.FailNow(t, "duration histogram was not collected")
	return metricdata.Histogram[float64]{}
}

func assertAttribute(
	t *testing.T,
	attributes []attribute.KeyValue,
	key attribute.Key,
	want string,
) {
	t.Helper()

	for _, current := range attributes {
		if current.Key == key {
			assert.Equal(t, want, current.Value.AsString())
			return
		}
	}

	assert.Fail(t, "attribute not found", "key: %s", key)
}

func assertSetAttribute(
	t *testing.T,
	attributes attribute.Set,
	key attribute.Key,
	want string,
) {
	t.Helper()

	value, ok := attributes.Value(key)
	require.True(t, ok, "attribute %s is missing", key)
	assert.Equal(t, want, value.AsString())
}
