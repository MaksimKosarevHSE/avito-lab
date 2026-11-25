package position

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionMetrics struct {
	staleChecks              metric.Int64Counter // queued duplicate dropped
	positionRequests         metric.Int64Counter // success error dropped
	positionRequestsDuration metric.Float64Histogram
	positionSaved            metric.Int64Counter
	rowsRead                 metric.Int64Histogram
	activeWorkers            atomic.Int64
}

type StaleCheckResult string

const (
	StaleCheckResultQueued    StaleCheckResult = "queued"
	StaleCheckResultDuplicate StaleCheckResult = "duplicate"
	StaleCheckResultDropped   StaleCheckResult = "dropped"
)

type PositionRequestResult string

const (
	PositionRequestResultSuccess PositionRequestResult = "success"
	PositionRequestResultError   PositionRequestResult = "error"
	PositionRequestResultDropped PositionRequestResult = "dropped"
)

func NewPositionsMetrics(
	meter metric.Meter,
	queueSize func() int,
) (*PositionMetrics, error) {
	if meter == nil {
		return nil, errors.New("meter must not be nil")
	}
	if queueSize == nil {
		return nil, errors.New("queueSize must not be nil")
	}
	staleChecks, err := meter.Int64Counter(
		"tripgo.stale_checks",
		metric.WithUnit("{check}"),
	)
	if err != nil {
		return nil, err
	}

	queueSize_, err := meter.Int64ObservableGauge("tripgo.position_requests.queue.size",
		metric.WithUnit("{task}"),
	)
	if err != nil {
		return nil, err
	}

	workersActive, err := meter.Int64ObservableGauge(
		"tripgo.position_requests.workers.active",
		metric.WithUnit("{worker}"),
	)
	if err != nil {
		return nil, err
	}

	positionRequests, err := meter.Int64Counter(
		"tripgo.position_requests",
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	positionSaved, err := meter.Int64Counter(
		"tripgo.positions.saved",
		metric.WithUnit("{position}"),
	)
	if err != nil {
		return nil, err
	}

	positionRequestsDuration, err := meter.Float64Histogram(
		"tripgo.position_requests.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.005,
			0.01,
			0.025,
			0.05,
			0.075,
			0.1,
			0.25,
			0.5,
			0.75,
			1,
			2.5,
			5,
			7.5,
			10,
		),
	)
	if err != nil {
		return nil, err
	}

	rowsRead, err := meter.Int64Histogram(
		"tripgo.stale_checks.rows_read",
		metric.WithUnit("{row}"),
		metric.WithExplicitBucketBoundaries(
			0,
			1,
			2,
			5,
			10,
			20,
			40,
			50,
			100,
			200,
			300,
			400,
			500,
			600,
			700,
			800,
			900,
			1000,
			5000,
			10000,
		))
	if err != nil {
		return nil, err
	}

	positionMetrics := &PositionMetrics{
		staleChecks:              staleChecks,
		positionRequests:         positionRequests,
		positionRequestsDuration: positionRequestsDuration,
		positionSaved:            positionSaved,
		rowsRead:                 rowsRead,
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(workersActive, positionMetrics.activeWorkers.Load())
		observer.ObserveInt64(queueSize_, int64(queueSize()))
		return nil
	},
		workersActive,
		queueSize_,
	)
	if err != nil {
		return nil, err
	}

	return positionMetrics, nil
}

func (p *PositionMetrics) AddStaleCheckResult(ctx context.Context, result StaleCheckResult) {
	p.staleChecks.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", string(result)),
	))
}

func (p *PositionMetrics) AddPositionRequestResult(ctx context.Context, result PositionRequestResult, duration time.Duration) {
	p.positionRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", string(result)),
	))

	if result == PositionRequestResultDropped {
		return
	}

	p.positionRequestsDuration.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributes(
			(attribute.String("result", string(result))),
		))
}

func (p *PositionMetrics) AddPositionSaved(ctx context.Context) {
	p.positionSaved.Add(ctx, 1)
}

func (p *PositionMetrics) RecordRowsRead(ctx context.Context, cnt int64) {
	p.rowsRead.Record(ctx, cnt)
}

func (p *PositionMetrics) IncrementActiveWorkers() {
	p.activeWorkers.Add(1)
}

func (p *PositionMetrics) DecrementActiveWorkers() {
	p.activeWorkers.Add(-1)
}
