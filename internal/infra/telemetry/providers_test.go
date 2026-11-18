package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOTLPEnvironment(t *testing.T) {
	clearOTLPEnvironment(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

	require.NoError(t, validateOTLPEnvironment())
}

func TestValidateOTLPEnvironmentRejectsMissingEndpoints(t *testing.T) {
	clearOTLPEnvironment(t)

	require.Error(t, validateOTLPEnvironment())
}

func TestValidateOTLPEnvironmentRejectsUnsupportedProtocol(t *testing.T) {
	clearOTLPEnvironment(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")

	require.Error(t, validateOTLPEnvironment())
}

func TestValidateOTLPEnvironmentRejectsInvalidEndpoint(t *testing.T) {
	clearOTLPEnvironment(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

	require.Error(t, validateOTLPEnvironment())
}

func TestValidateOTLPEnvironmentAcceptsSignalSpecificEndpoints(t *testing.T) {
	clearOTLPEnvironment(t)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:4318/v1/traces")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "http://localhost:4318/v1/metrics")

	require.NoError(t, validateOTLPEnvironment())
}

func clearOTLPEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
	} {
		t.Setenv(name, "")
	}
}
