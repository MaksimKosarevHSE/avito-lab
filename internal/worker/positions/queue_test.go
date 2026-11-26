package positions_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
)

func TestPriorityQueueTakesOldestTripFirst(t *testing.T) {
	queue := positions.NewPriorityQueue(3)
	now := time.Now()

	newest := selectedTrip(now.Add(-time.Minute))
	oldest := selectedTrip(now.Add(-3 * time.Minute))
	middle := selectedTrip(now.Add(-2 * time.Minute))

	require.Equal(t, positions.PushStatusQueued, queue.Enqueue(newest).Status)
	require.Equal(t, positions.PushStatusQueued, queue.Enqueue(oldest).Status)
	require.Equal(t, positions.PushStatusQueued, queue.Enqueue(middle).Status)

	for _, expected := range []positions.SelectedTrip{oldest, middle, newest} {
		actual, ok := queue.Take()
		require.True(t, ok)
		assert.Equal(t, expected.TripID, actual.TripID)
		queue.Complete(actual.TripID)
	}
}

func TestPriorityQueueEvictsNewestTripWhenOlderTripArrives(t *testing.T) {
	queue := positions.NewPriorityQueue(2)
	now := time.Now()

	middle := selectedTrip(now.Add(-2 * time.Minute))
	newest := selectedTrip(now.Add(-time.Minute))
	oldest := selectedTrip(now.Add(-3 * time.Minute))

	queue.Enqueue(middle)
	queue.Enqueue(newest)

	result := queue.Enqueue(oldest)

	require.Equal(t, positions.PushStatusQueued, result.Status)
	require.NotNil(t, result.Removed)
	assert.Equal(t, newest.TripID, result.Removed.TripID)

	tooFresh := selectedTrip(now)
	assert.Equal(t, positions.PushStatusDropped, queue.Enqueue(tooFresh).Status)
}

func selectedTrip(lastTrackedAt time.Time) positions.SelectedTrip {
	return positions.SelectedTrip{
		TripID:        uuid.New(),
		DriverID:      uuid.New(),
		LastTrackedAt: lastTrackedAt,
	}
}
