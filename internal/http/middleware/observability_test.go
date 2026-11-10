package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestRouteTemplateDoesNotUseConcretePath(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/api/v1/trips/{tripID}", func(http.ResponseWriter, *http.Request) {})

	first := httptest.NewRequest(http.MethodGet, "/api/v1/trips/first-secret-id", nil)
	second := httptest.NewRequest(http.MethodGet, "/api/v1/trips/second-secret-id", nil)
	unmatched := httptest.NewRequest(http.MethodGet, "/private/unknown-value", nil)

	assert.Equal(t, "/api/v1/trips/{tripID}", RouteTemplate(router, first))
	assert.Equal(t, "/api/v1/trips/{tripID}", RouteTemplate(router, second))
	assert.Equal(t, unmatchedRoute, RouteTemplate(router, unmatched))
}

func TestRouteTemplateUsesRawPathLikeChi(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/files/{value}", func(http.ResponseWriter, *http.Request) {})

	request := httptest.NewRequest(http.MethodGet, "http://example.com/files/a%2Fb", nil)
	require.NotEmpty(t, request.URL.RawPath)
	assert.Equal(t, "/files/{value}", RouteTemplate(router, request))
}

func TestObservabilityMiddleware(t *testing.T) {
	logHandler := &recordingLogHandler{}
	logger := slog.New(logHandler)
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() { require.NoError(t, tracerProvider.Shutdown(context.Background())) })

	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(previousPropagator) })

	httpMetrics, err := NewHTTPMetrics(metricnoop.NewMeterProvider().Meter("test"))
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Get("/api/v1/trips/{tripID}", func(w http.ResponseWriter, r *http.Request) {
		switch chi.URLParam(r, "tripID") {
		case "failed":
			http.Error(w, "safe public error", http.StatusInternalServerError)
		case "panicked":
			panic("private panic detail")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	handler := Recover(logger)(
		Tracing(tracerProvider)(
			Logging(logger)(
				Metrics(httpMetrics)(router),
			),
		),
	)

	const parentTraceID = "0af7651916cd43dd8448eb211c80319c"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/success", nil)
	request.Header.Set("traceparent", "00-"+parentTraceID+"-b7ad6b7169203331-01")
	success := httptest.NewRecorder()
	handler.ServeHTTP(success, request)
	assert.Equal(t, http.StatusNoContent, success.Code)

	failed := httptest.NewRecorder()
	handler.ServeHTTP(
		failed,
		httptest.NewRequest(http.MethodGet, "/api/v1/trips/failed", nil),
	)
	assert.Equal(t, http.StatusInternalServerError, failed.Code)

	panicked := httptest.NewRecorder()
	handler.ServeHTTP(
		panicked,
		httptest.NewRequest(http.MethodGet, "/api/v1/trips/panicked", nil),
	)
	assert.Equal(t, http.StatusInternalServerError, panicked.Code)
	assert.Equal(t, "application/problem+json", panicked.Header().Get("Content-Type"))
	assert.NotContains(t, panicked.Body.String(), "private panic detail")
	var problem internalProblem
	require.NoError(t, json.NewDecoder(panicked.Body).Decode(&problem))
	assert.Equal(t, "internal_error", problem.Code)

	spans := spanRecorder.Ended()
	require.Len(t, spans, 3)
	for _, span := range spans {
		assert.Equal(t, "GET /api/v1/trips/{tripID}", span.Name())
		assert.Equal(t, "/api/v1/trips/{tripID}", spanAttribute(span.Attributes(), "http.route"))
	}
	assert.Equal(t, parentTraceID, spans[0].Parent().TraceID().String())
	assert.Equal(t, codes.Unset, spans[0].Status().Code)
	assert.Equal(t, "500", spanAttribute(spans[1].Attributes(), "error.type"))
	assert.Equal(t, "panic", spanAttribute(spans[2].Attributes(), "error.type"))

	logs := logHandler.recordsCopy()
	requestLogs := recordsWithMessage(logs, "request handled")
	require.Len(t, requestLogs, 3)
	assert.Equal(t, slog.LevelInfo, requestLogs[0].level)
	assert.EqualValues(t, http.StatusNoContent, requestLogs[0].attrs["status"])
	assert.Equal(t, slog.LevelError, requestLogs[1].level)
	assert.EqualValues(t, http.StatusInternalServerError, requestLogs[1].attrs["status"])
	assert.Equal(t, slog.LevelError, requestLogs[2].level)
	assert.EqualValues(t, http.StatusInternalServerError, requestLogs[2].attrs["status"])
	for _, record := range requestLogs {
		assert.Equal(t, "/api/v1/trips/{tripID}", record.attrs["route"])
		assert.NotContains(t, record.attrs["route"], "success")
		assert.True(t, record.spanContext.IsValid())
	}

	panicLogs := recordsWithMessage(logs, "panic recovered")
	require.Len(t, panicLogs, 1)
	assert.True(t, panicLogs[0].spanContext.IsValid())
	assert.Equal(t, spans[2].SpanContext().TraceID(), panicLogs[0].spanContext.TraceID())
	assert.Contains(t, panicLogs[0].attrs["stack"], "TestObservabilityMiddleware")
}

func TestHTTPMetricsUseOneSeriesForRouteTemplate(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, meterProvider.Shutdown(context.Background())) })

	httpMetrics, err := NewHTTPMetrics(meterProvider.Meter("test"))
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Get("/api/v1/trips/{tripID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := Metrics(httpMetrics)(router)

	for _, tripID := range []string{"first-id", "second-id"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID, nil),
		)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}

	var resourceMetrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &resourceMetrics))

	durationMetric := metricByName(resourceMetrics, "http.server.request.duration")
	require.NotNil(t, durationMetric)
	duration, ok := durationMetric.Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	require.Len(t, duration.DataPoints, 1)
	assert.Equal(t, uint64(2), duration.DataPoints[0].Count)
	assert.Equal(
		t,
		"/api/v1/trips/{tripID}",
		attributeValue(duration.DataPoints[0].Attributes.ToSlice(), "http.route"),
	)

	activeMetric := metricByName(resourceMetrics, "http.server.active_requests")
	require.NotNil(t, activeMetric)
	active, ok := activeMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	require.Len(t, active.DataPoints, 1)
	assert.Equal(t, int64(0), active.DataPoints[0].Value)
}

func TestHTTPMetricsBoundsUnknownMethods(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, meterProvider.Shutdown(context.Background())) })

	httpMetrics, err := NewHTTPMetrics(meterProvider.Meter("test"))
	require.NoError(t, err)
	handler := Metrics(httpMetrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, method := range []string{"CUSTOM-ONE", "CUSTOM-TWO"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "/", nil))
	}

	var resourceMetrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &resourceMetrics))
	duration, ok := metricByName(
		resourceMetrics,
		"http.server.request.duration",
	).Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	require.Len(t, duration.DataPoints, 1)
	assert.Equal(t, uint64(2), duration.DataPoints[0].Count)
	assert.Equal(
		t,
		"_OTHER",
		attributeValue(duration.DataPoints[0].Attributes.ToSlice(), "http.request.method"),
	)
}

func TestMetricsDecrementActiveRequestsAfterPanic(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, meterProvider.Shutdown(context.Background())) })

	httpMetrics, err := NewHTTPMetrics(meterProvider.Meter("test"))
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recover(logger)(Metrics(httpMetrics)(router))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	var resourceMetrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &resourceMetrics))

	activeMetric := metricByName(resourceMetrics, "http.server.active_requests")
	require.NotNil(t, activeMetric)
	active := activeMetric.Data.(metricdata.Sum[int64])
	require.Len(t, active.DataPoints, 1)
	assert.Equal(t, int64(0), active.DataPoints[0].Value)

	durationMetric := metricByName(resourceMetrics, "http.server.request.duration")
	require.NotNil(t, durationMetric)
	duration := durationMetric.Data.(metricdata.Histogram[float64])
	require.Len(t, duration.DataPoints, 1)
	assert.EqualValues(
		t,
		http.StatusInternalServerError,
		attributeValue(duration.DataPoints[0].Attributes.ToSlice(), "http.response.status_code"),
	)
}

func TestLoggingKeepsClientErrorsAtInfoLevel(t *testing.T) {
	logHandler := &recordingLogHandler{}
	logger := slog.New(logHandler)
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/private-id", nil))

	records := recordsWithMessage(logHandler.recordsCopy(), "request handled")
	require.Len(t, records, 1)
	assert.Equal(t, slog.LevelInfo, records[0].level)
	assert.Equal(t, unmatchedRoute, records[0].attrs["route"])
	assert.NotContains(t, records[0].attrs["route"], "private-id")
}

type recordedLog struct {
	level       slog.Level
	message     string
	attrs       map[string]any
	spanContext trace.SpanContext
}

type recordingLogHandler struct {
	mu      sync.Mutex
	records []recordedLog
}

func (h *recordingLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingLogHandler) Handle(ctx context.Context, record slog.Record) error {
	attrs := make(map[string]any)
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})

	h.mu.Lock()
	h.records = append(h.records, recordedLog{
		level:       record.Level,
		message:     record.Message,
		attrs:       attrs,
		spanContext: trace.SpanContextFromContext(ctx),
	})
	h.mu.Unlock()

	return nil
}

func (h *recordingLogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *recordingLogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *recordingLogHandler) recordsCopy() []recordedLog {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]recordedLog(nil), h.records...)
}

func recordsWithMessage(records []recordedLog, message string) []recordedLog {
	result := make([]recordedLog, 0, len(records))
	for _, record := range records {
		if record.message == message {
			result = append(result, record)
		}
	}

	return result
}

func spanAttribute(attrs []attribute.KeyValue, key string) string {
	value := attributeValue(attrs, key)
	stringValue, _ := value.(string)
	return stringValue
}

func attributeValue(attrs []attribute.KeyValue, key string) any {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return attr.Value.AsInterface()
		}
	}

	return nil
}

func metricByName(resourceMetrics metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for metricIndex := range scopeMetrics.Metrics {
			if scopeMetrics.Metrics[metricIndex].Name == name {
				return &scopeMetrics.Metrics[metricIndex]
			}
		}
	}

	return nil
}

func TestRecoverDoesNotExposePanicInJSON(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("database password")
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, strings.Contains(recorder.Body.String(), "database password"))
}

func TestRecoverDoesNotAppendProblemAfterResponseWasCommitted(t *testing.T) {
	logHandler := &recordingLogHandler{}
	logger := slog.New(logHandler)
	handler := Recover(logger)(Logging(logger)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("partial response"))
			panic("failure after write")
		},
	)))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.Equal(t, "partial response", recorder.Body.String())
	requestLogs := recordsWithMessage(logHandler.recordsCopy(), "request handled")
	require.Len(t, requestLogs, 1)
	assert.Equal(t, slog.LevelError, requestLogs[0].level)
	assert.EqualValues(t, http.StatusAccepted, requestLogs[0].attrs["status"])
}
