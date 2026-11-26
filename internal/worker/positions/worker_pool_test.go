package positions_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
)

func TestWorkerPoolUsesConfiguredNumberOfWorkers(t *testing.T) {
	const workers = 2

	queue := positions.NewPriorityQueue(4)
	now := time.Now()
	for i := range 4 {
		queue.Enqueue(selectedTrip(now.Add(time.Duration(i) * time.Minute)))
	}
	queue.Close()

	pusher := newBlockingPusher(4)
	t.Cleanup(pusher.releaseAll)
	pool := newWorkerPool(workers, queue, pusher)
	done := make(chan struct{})
	go func() {
		pool.Start(t.Context())
		close(done)
	}()

	for range workers {
		receive(t, pusher.started)
	}
	select {
	case <-pusher.started:
		t.Fatal("more workers started processing than configured")
	default:
	}

	pusher.releaseAll()
	waitDone(t, done)

	assert.EqualValues(t, 4, pusher.calls.Load())
}

func TestWorkerPoolContinuesAfterPushError(t *testing.T) {
	queue := positions.NewPriorityQueue(2)
	first := selectedTrip(time.Now().Add(-time.Minute))
	second := selectedTrip(time.Now())
	queue.Enqueue(first)
	queue.Enqueue(second)
	queue.Close()

	pusher := &scriptedPusher{errors: []error{errors.New("push unavailable"), nil}}
	pool := newWorkerPool(1, queue, pusher)
	pool.Start(t.Context())

	require.Len(t, pusher.tripIDs, 2)
	assert.Equal(t, first.TripID, pusher.tripIDs[0])
	assert.Equal(t, second.TripID, pusher.tripIDs[1])
}

func TestCancellingQueuedTripPreventsPush(t *testing.T) {
	queue := positions.NewPriorityQueue(1)
	trip := selectedTrip(time.Now())
	queue.Enqueue(trip)

	require.True(t, queue.Cancel(trip.TripID))
	queue.Close()

	pusher := &scriptedPusher{}
	newWorkerPool(1, queue, pusher).Start(t.Context())

	assert.Empty(t, pusher.tripIDs)
}

func TestWorkerPoolStopsAfterContextCancellation(t *testing.T) {
	queue := positions.NewPriorityQueue(1)
	pool := newWorkerPool(1, queue, &scriptedPusher{})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		pool.Start(ctx)
		close(done)
	}()

	cancel()
	waitDone(t, done)
}

func newWorkerPool(
	workers int32,
	queue *positions.PriorityQueue,
	pusher positions.PositionPusher,
) *positions.WorkerPool {
	return positions.NewWorkerPool(
		workers,
		queue,
		pusher,
		slog.New(slog.DiscardHandler),
		workerMetrics{},
		otel.Tracer("worker-pool-test"),
	)
}

type workerMetrics struct{}

func (workerMetrics) IncrementActiveWorkers() {}

func (workerMetrics) DecrementActiveWorkers() {}

func (workerMetrics) AddPositionRequestResult(
	context.Context,
	positions.PositionRequestResult,
	time.Duration,
) {
}

type scriptedPusher struct {
	errors  []error
	tripIDs []uuid.UUID
}

func (p *scriptedPusher) PushPosition(_ context.Context, tripID, _ uuid.UUID) error {
	p.tripIDs = append(p.tripIDs, tripID)
	if len(p.errors) == 0 {
		return nil
	}
	err := p.errors[0]
	p.errors = p.errors[1:]
	return err
}

type blockingPusher struct {
	started   chan struct{}
	release   chan struct{}
	releaseMu sync.Once
	calls     atomic.Int32
}

func newBlockingPusher(tasks int) *blockingPusher {
	return &blockingPusher{
		started: make(chan struct{}, tasks),
		release: make(chan struct{}),
	}
}

func (p *blockingPusher) PushPosition(context.Context, uuid.UUID, uuid.UUID) error {
	p.calls.Add(1)
	p.started <- struct{}{}
	<-p.release
	return nil
}

func (p *blockingPusher) releaseAll() {
	p.releaseMu.Do(func() { close(p.release) })
}

func receive[T any](t *testing.T, values <-chan T) T {
	t.Helper()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case value := <-values:
		return value
	case <-timer.C:
		t.Fatal("timed out waiting for test event")
		var zero T
		return zero
	}
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	receive(t, done)
}
