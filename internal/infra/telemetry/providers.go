package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	metricexport "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	traceexport "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const (
	serviceNamespace      = "tripgo"
	deploymentEnvironment = "local"
)

type Providers struct {
	Traces  *sdktrace.TracerProvider
	Metrics *sdkmetric.MeterProvider
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	serviceName string,
	serviceVersion string,
) (*Providers, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("service name is required")
	}
	if strings.TrimSpace(serviceVersion) == "" {
		return nil, errors.New("service version is required")
	}
	if err := validateOTLPEnvironment(); err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.ServiceNamespace(serviceNamespace),
			attribute.String("deployment.environment", deploymentEnvironment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	spanExporter, err := traceexport.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	metricExporter, err := metricexport.New(ctx)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("create OTLP metric exporter: %w", err),
			spanExporter.Shutdown(ctx),
		)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(spanExporter),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Warn("OpenTelemetry export failed", "error", err)
	}))

	return &Providers{Traces: tracerProvider, Metrics: meterProvider}, nil
}

func (p *Providers) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}

	var metricErr error
	if p.Metrics != nil {
		metricErr = p.Metrics.Shutdown(ctx)
	}

	var traceErr error
	if p.Traces != nil {
		traceErr = p.Traces.Shutdown(ctx)
	}

	return errors.Join(metricErr, traceErr)
}

func validateOTLPEnvironment() error {
	for _, name := range []string{
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
	} {
		if protocol := strings.TrimSpace(os.Getenv(name)); protocol != "" && protocol != "http/protobuf" {
			return fmt.Errorf("%s: unsupported OTLP protocol %q", name, protocol)
		}
	}

	commonEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	traceEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"))
	metricEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"))
	if commonEndpoint == "" && (traceEndpoint == "" || metricEndpoint == "") {
		return errors.New("OTEL_EXPORTER_OTLP_ENDPOINT or both signal-specific endpoints are required")
	}

	for name, endpoint := range map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT":         commonEndpoint,
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  traceEndpoint,
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": metricEndpoint,
	} {
		if endpoint == "" {
			continue
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%s: invalid HTTP endpoint %q", name, endpoint)
		}
	}

	return nil
}
