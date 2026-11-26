//go:build integration

package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	positiontest "github.com/MaksimKosarevHSE/avito-lab/internal/testkit/position"
)

type SelectionSuite struct {
	suite.Suite

	app   *positiontest.App
	steps *positiontest.Steps
}

func (s *SelectionSuite) SetupSuite() {
	s.app = positiontest.NewApp(s.T())
}

func (s *SelectionSuite) SetupTest() {
	s.app.ResetTrips(s.T())
	s.steps = positiontest.NewSteps(s.T(), s.app)
}

func (s *SelectionSuite) TestSelectsOnlyActiveStaleTrips() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	stale := s.steps.GivenActiveTrip(now.Add(-2 * time.Hour))
	fresh := s.steps.GivenActiveTrip(now.Add(-2 * time.Hour))
	s.steps.GivenLastPositionAt(fresh.ID, now)
	s.steps.GivenCompletedTrip(now.Add(-2 * time.Hour))

	result := s.steps.WhenStaleTripsAreSelected(time.Hour, 100, nil)

	s.steps.ThenOnlyTripsAreSelected(result, stale.ID)
}

func (s *SelectionSuite) TestKeysetPaginationDoesNotLoseTripsWithSameTimestamp() {
	const tripCount = 5
	startedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	expected := make([]uuid.UUID, tripCount)
	for i := range tripCount {
		expected[i] = s.steps.GivenActiveTrip(startedAt).ID
	}

	var actual []uuid.UUID
	next := s.steps.WhenStaleTripsAreSelected(time.Hour, 2, nil)
	for {
		for _, trip := range next.SelectedTrips {
			actual = append(actual, trip.TripID)
		}
		if len(next.SelectedTrips) < 2 {
			break
		}
		next = s.steps.WhenStaleTripsAreSelected(time.Hour, 2, next.NextPtr)
	}

	require.Len(s.T(), actual, tripCount)
	assert.ElementsMatch(s.T(), expected, actual)
	assert.Len(s.T(), uniqueIDs(actual), tripCount)
}

func uniqueIDs(ids []uuid.UUID) map[uuid.UUID]struct{} {
	result := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func TestSelectionSuite(t *testing.T) {
	suite.Run(t, new(SelectionSuite))
}
