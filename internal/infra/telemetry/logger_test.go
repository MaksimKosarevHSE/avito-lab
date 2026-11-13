package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestLoggerAddsSpanContext(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output, slog.LevelInfo, "trip-service")

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:  trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	logger.InfoContext(ctx, "request completed", "status", 200)

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, "trip-service", record["service"])
	require.Equal(t, spanContext.TraceID().String(), record["trace_id"])
	require.Equal(t, spanContext.SpanID().String(), record["span_id"])
	require.Equal(t, "request completed", record["msg"])
}

func TestLoggerAddsEmptySpanIdentifiersOutsideSpan(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output, slog.LevelInfo, "trip-service")

	logger.InfoContext(context.Background(), "component started")

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, "", record["trace_id"])
	require.Equal(t, "", record["span_id"])
}

func TestLoggerHonorsLevel(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output, slog.LevelWarn, "trip-service")

	logger.InfoContext(context.Background(), "hidden")
	logger.WarnContext(context.Background(), "visible")

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, "visible", record["msg"])
}
