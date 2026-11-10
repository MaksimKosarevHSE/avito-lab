package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

type HTTPMetrics struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

func NewHTTPMetrics(meter metric.Meter) (*HTTPMetrics, error) {
	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of inbound HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP request duration histogram: %w", err)
	}

	active, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of active inbound HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active HTTP requests counter: %w", err)
	}

	return &HTTPMetrics{duration: duration, active: active}, nil
}

func Metrics(
	instruments *HTTPMetrics,
	routes ...chi.Routes,
) func(http.Handler) http.Handler {
	if instruments == nil {
		panic("metrics middleware requires HTTP metrics")
	}

	routeTable := optionalRoutes(routes)

	return func(next http.Handler) http.Handler {
		routeTable := routesForHandler(routeTable, next)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := RouteTemplate(routeTable, r)
			requestAttrs := []attribute.KeyValue{
				semconv.HTTPRequestMethodKey.String(metricMethod(r.Method)),
				semconv.HTTPRoute(route),
			}

			startedAt := time.Now()
			status := http.StatusOK
			instruments.active.Add(
				r.Context(),
				1,
				metric.WithAttributes(requestAttrs...),
			)
			finish := func(finalStatus int) {
				instruments.active.Add(
					r.Context(),
					-1,
					metric.WithAttributes(requestAttrs...),
				)
				durationAttrs := append([]attribute.KeyValue{}, requestAttrs...)
				durationAttrs = append(
					durationAttrs,
					semconv.HTTPResponseStatusCode(finalStatus),
				)
				instruments.duration.Record(
					r.Context(),
					time.Since(startedAt).Seconds(),
					metric.WithAttributes(durationAttrs...),
				)
			}

			defer func() {
				panicValue := recover()
				if panicValue != nil {
					if !requestStateFromContext(r.Context()).addPanicFinalizer(finish) {
						finish(http.StatusInternalServerError)
					}
					panic(panicValue)
				}

				finish(status)
			}()

			observed := httpsnoop.CaptureMetrics(next, w, r)
			status = observed.Code
		})

		return carryRoutes(handler, routeTable)
	}
}

func metricMethod(method string) string {
	switch method {
	case http.MethodConnect,
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
		http.MethodTrace:
		return method
	default:
		return "_OTHER"
	}
}
