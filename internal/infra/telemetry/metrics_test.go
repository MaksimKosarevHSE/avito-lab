package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestTripMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	metrics, err := NewTripMetrics(provider.Meter("test"), 2)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, metrics.Unregister())
	})

	ctx := context.Background()
	metrics.TripCreated(ctx)
	metrics.TripCompleted(ctx)
	metrics.TripCompleted(ctx)

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	require.EqualValues(t, 1, int64SumValue(t, data, "tripgo.trips.created"))
	require.EqualValues(t, 2, int64SumValue(t, data, "tripgo.trips.completed"))
	require.EqualValues(t, 1, int64GaugeValue(t, data, "tripgo.trips.active"))
}

func TestTripMetricsDoesNotMakeActiveCountNegative(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	metrics, err := NewTripMetrics(provider.Meter("test"), 0)
	require.NoError(t, err)
	metrics.TripCompleted(context.Background())

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	require.EqualValues(t, 0, int64GaugeValue(t, data, "tripgo.trips.active"))
}

func TestTripMetricsRejectsNegativeInitialCount(t *testing.T) {
	provider := sdkmetric.NewMeterProvider()
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	_, err := NewTripMetrics(provider.Meter("test"), -1)
	require.Error(t, err)
}

func int64SumValue(t *testing.T, data metricdata.ResourceMetrics, name string) int64 {
	t.Helper()

	metric := findMetric(t, data, name)
	sum, ok := metric.Data.(metricdata.Sum[int64])
	require.True(t, ok, "%s is not an int64 sum", name)
	require.Len(t, sum.DataPoints, 1)

	return sum.DataPoints[0].Value
}

func int64GaugeValue(t *testing.T, data metricdata.ResourceMetrics, name string) int64 {
	t.Helper()

	metric := findMetric(t, data, name)
	gauge, ok := metric.Data.(metricdata.Gauge[int64])
	require.True(t, ok, "%s is not an int64 gauge", name)
	require.Len(t, gauge.DataPoints, 1)

	return gauge.DataPoints[0].Value
}

func findMetric(t *testing.T, data metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()

	for _, scope := range data.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == name {
				return metric
			}
		}
	}

	require.FailNow(t, "metric not found", name)
	return metricdata.Metrics{}
}
