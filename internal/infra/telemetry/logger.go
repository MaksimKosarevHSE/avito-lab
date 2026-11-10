package telemetry

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type TraceHandler struct {
	next slog.Handler
}

func NewTraceHandler(next slog.Handler) slog.Handler {
	return TraceHandler{next: next}
}

func (h TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h TraceHandler) Handle(ctx context.Context, record slog.Record) error {
	record = record.Clone()

	traceID := ""
	spanID := ""
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
		spanID = spanContext.SpanID().String()
	}

	record.AddAttrs(
		slog.String("trace_id", traceID),
		slog.String("span_id", spanID),
	)

	return h.next.Handle(ctx, record)
}

func (h TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return TraceHandler{next: h.next.WithAttrs(attrs)}
}

func (h TraceHandler) WithGroup(name string) slog.Handler {
	return TraceHandler{next: h.next.WithGroup(name)}
}

func NewLogger(writer io.Writer, level slog.Level, serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})

	return slog.New(NewTraceHandler(handler)).With("service", serviceName)
}
