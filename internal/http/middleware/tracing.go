package middleware

import (
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(tracerProvider trace.TracerProvider) func(http.Handler) http.Handler {
	if tracerProvider == nil {
		panic("tracing middleware requires a tracer provider")
	}

	return func(next http.Handler) http.Handler {
		withRoute := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := trace.SpanFromContext(r.Context())
			if state := requestStateFromContext(r.Context()); state != nil {
				state.setSpanContext(span.SpanContext())
			}

			status := http.StatusOK
			defer func() {
				panicValue := recover()
				route := routePattern(r)
				span.SetName(r.Method + " " + route)
				span.SetAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.HTTPRoute(route),
				)

				if panicValue != nil {
					span.SetStatus(codes.Error, "panic")
					span.SetAttributes(attribute.String("error.type", "panic"))
					panic(panicValue)
				}

				if status >= http.StatusInternalServerError {
					span.SetStatus(codes.Error, http.StatusText(status))
					span.SetAttributes(attribute.String("error.type", strconv.Itoa(status)))
				}
			}()

			observed := httpsnoop.CaptureMetrics(next, w, r)
			status = observed.Code
		})

		handler := otelhttp.NewHandler(
			withRoute,
			"HTTP",
			otelhttp.WithTracerProvider(tracerProvider),
			otelhttp.WithMeterProvider(metricnoop.NewMeterProvider()),
			otelhttp.WithPropagators(otel.GetTextMapPropagator()),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + routePattern(r)
			}),
		)

		return handler
	}
}
