package telemetry

import (
	"context"
	"errors"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
)

type TripMetrics struct {
	created      metric.Int64Counter
	completed    metric.Int64Counter
	active       atomic.Int64
	registration metric.Registration
}

func NewTripMetrics(meter metric.Meter, initialActive int64) (*TripMetrics, error) {
	if meter == nil {
		return nil, errors.New("meter is required")
	}
	if initialActive < 0 {
		return nil, errors.New("initial active trips count cannot be negative")
	}

	created, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithDescription("Number of successfully created trips"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	completed, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithDescription("Number of successfully completed trips"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	activeGauge, err := meter.Int64ObservableGauge(
		"tripgo.trips.active",
		metric.WithDescription("Current number of active trips"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	metrics := &TripMetrics{
		created:   created,
		completed: completed,
	}
	metrics.active.Store(initialActive)

	registration, err := meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			observer.ObserveInt64(activeGauge, metrics.active.Load())
			return nil
		},
		activeGauge,
	)
	if err != nil {
		return nil, err
	}
	metrics.registration = registration

	return metrics, nil
}

func (m *TripMetrics) TripCreated(ctx context.Context) {
	m.created.Add(ctx, 1)
	m.active.Add(1)
}

func (m *TripMetrics) TripCompleted(ctx context.Context) {
	m.completed.Add(ctx, 1)
	m.decrementActive()
}

func (m *TripMetrics) Unregister() error {
	if m == nil || m.registration == nil {
		return nil
	}

	return m.registration.Unregister()
}

func (m *TripMetrics) decrementActive() {
	for {
		current := m.active.Load()
		if current == 0 {
			return
		}
		if m.active.CompareAndSwap(current, current-1) {
			return
		}
	}
}
