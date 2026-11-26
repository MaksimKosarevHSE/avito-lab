package positions_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
)

const testBatchSize = 100

func TestSelectionJobDoesNotQueueSameTripTwice(t *testing.T) {
	trip := selectedTrip(time.Now().Add(-time.Hour))
	selector := newScriptedSelector(
		positions.FetchForPushResult{SelectedTrips: []positions.SelectedTrip{trip}},
		positions.FetchForPushResult{SelectedTrips: []positions.SelectedTrip{trip}},
	)
	queue := positions.NewPriorityQueue(2)
	metrics := newJobMetrics()
	clock := newManualClock()
	job := newSelectionJob(clock, selector, queue, metrics)
	runSelectionJob(t, job, clock)

	clock.tick()
	require.EqualValues(t, 1, receive(t, metrics.rowsRead))
	clock.tick()
	require.EqualValues(t, 1, receive(t, metrics.rowsRead))

	assert.Equal(t, positions.StaleCheckResultQueued, receive(t, metrics.staleResults))
	assert.Equal(t, positions.StaleCheckResultDuplicate, receive(t, metrics.staleResults))
	assert.Equal(t, 1, queue.Len())
}

func TestSelectionJobReadsAllBatches(t *testing.T) {
	trips := selectedTrips(250)
	firstPtr := positions.FetchForPushPtr{
		LastTrackedAt: trips[99].LastTrackedAt,
		TripID:        trips[99].TripID,
	}
	secondPtr := positions.FetchForPushPtr{
		LastTrackedAt: trips[199].LastTrackedAt,
		TripID:        trips[199].TripID,
	}
	selector := newScriptedSelector(
		positions.FetchForPushResult{SelectedTrips: trips[:100], NextPtr: &firstPtr},
		positions.FetchForPushResult{SelectedTrips: trips[100:200], NextPtr: &secondPtr},
		positions.FetchForPushResult{SelectedTrips: trips[200:]},
	)
	queue := positions.NewPriorityQueue(250)
	metrics := newJobMetrics()
	clock := newManualClock()
	job := newSelectionJob(clock, selector, queue, metrics)
	runSelectionJob(t, job, clock)

	clock.tick()
	require.EqualValues(t, 250, receive(t, metrics.rowsRead))

	require.Len(t, selector.calls, 3)
	assert.Nil(t, selector.calls[0])
	assert.Equal(t, firstPtr, *selector.calls[1])
	assert.Equal(t, secondPtr, *selector.calls[2])
	assert.Equal(t, 250, queue.Len())
}

func TestSelectionJobStopsCurrentScanWhenQueueIsFull(t *testing.T) {
	trips := selectedTrips(250)
	selector := newScriptedSelector(
		positions.FetchForPushResult{SelectedTrips: trips[:100]},
		positions.FetchForPushResult{SelectedTrips: trips[100:200]},
		positions.FetchForPushResult{SelectedTrips: trips[200:]},
	)
	queue := positions.NewPriorityQueue(50)
	metrics := newJobMetrics()
	clock := newManualClock()
	job := newSelectionJob(clock, selector, queue, metrics)
	runSelectionJob(t, job, clock)

	clock.tick()
	require.EqualValues(t, 100, receive(t, metrics.rowsRead))

	assert.Len(t, selector.calls, 1)
	assert.Equal(t, 50, queue.Len())
	assert.Contains(t, metrics.snapshotStaleResults(), positions.StaleCheckResultDropped)
}

func newSelectionJob(
	clock positions.Clock,
	selector positions.TripBatchSelector,
	queue *positions.PriorityQueue,
	metrics positions.JobPositionMetrics,
) *positions.SelectionJob {
	return positions.NewSelectionJob(
		time.Minute,
		testBatchSize,
		time.Minute,
		clock,
		selector,
		queue,
		slog.New(slog.DiscardHandler),
		metrics,
		otel.Tracer("selection-job-test"),
	)
}

func runSelectionJob(
	t *testing.T,
	job *positions.SelectionJob,
	clock *manualClock,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		job.Run(ctx)
		close(done)
	}()
	receive(t, clock.created)

	t.Cleanup(func() {
		cancel()
		waitDone(t, done)
		require.True(t, clock.ticker.stopped)
	})
}

func selectedTrips(count int) []positions.SelectedTrip {
	result := make([]positions.SelectedTrip, count)
	startedAt := time.Now().Add(-time.Duration(count) * time.Minute)
	for i := range result {
		result[i] = selectedTrip(startedAt.Add(time.Duration(i) * time.Minute))
	}
	return result
}

type manualClock struct {
	ticker  *manualTicker
	created chan time.Duration
}

func newManualClock() *manualClock {
	return &manualClock{
		ticker:  &manualTicker{ticks: make(chan time.Time, 1)},
		created: make(chan time.Duration, 1),
	}
}

func (c *manualClock) NewTicker(interval time.Duration) positions.Ticker {
	c.created <- interval
	return c.ticker
}

func (c *manualClock) tick() {
	c.ticker.ticks <- time.Now()
}

type manualTicker struct {
	ticks   chan time.Time
	stopped bool
}

func (t *manualTicker) C() <-chan time.Time {
	return t.ticks
}

func (t *manualTicker) Stop() {
	t.stopped = true
}

type scriptedSelector struct {
	results []positions.FetchForPushResult
	calls   []*positions.FetchForPushPtr
}

func newScriptedSelector(results ...positions.FetchForPushResult) *scriptedSelector {
	return &scriptedSelector{results: results}
}

func (s *scriptedSelector) FetchForPushQuery(
	_ context.Context,
	_ time.Duration,
	_ int32,
	ptr *positions.FetchForPushPtr,
) (positions.FetchForPushResult, error) {
	s.calls = append(s.calls, ptr)
	if len(s.results) == 0 {
		return positions.FetchForPushResult{}, nil
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

type jobMetrics struct {
	staleResults chan positions.StaleCheckResult
	rowsRead     chan int64
	allStale     []positions.StaleCheckResult
}

func newJobMetrics() *jobMetrics {
	return &jobMetrics{
		staleResults: make(chan positions.StaleCheckResult, 300),
		rowsRead:     make(chan int64, 1),
	}
}

func (m *jobMetrics) AddStaleCheckResult(
	_ context.Context,
	result positions.StaleCheckResult,
) {
	m.allStale = append(m.allStale, result)
	m.staleResults <- result
}

func (*jobMetrics) AddPositionRequestResult(
	context.Context,
	positions.PositionRequestResult,
	time.Duration,
) {
}

func (m *jobMetrics) RecordRowsRead(_ context.Context, count int64) {
	m.rowsRead <- count
}

func (m *jobMetrics) snapshotStaleResults() []positions.StaleCheckResult {
	return append([]positions.StaleCheckResult(nil), m.allStale...)
}
