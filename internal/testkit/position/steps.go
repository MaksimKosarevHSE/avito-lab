//go:build integration

package positiontest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainposition "github.com/MaksimKosarevHSE/avito-lab/internal/domain/position"
	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	positioncreate "github.com/MaksimKosarevHSE/avito-lab/internal/usecase/position/create"
	"github.com/MaksimKosarevHSE/avito-lab/internal/worker/positions"
)

type Steps struct {
	t   testing.TB
	app *App
}

func NewSteps(t testing.TB, app *App) *Steps {
	t.Helper()
	require.NotNil(t, app)
	return &Steps{t: t, app: app}
}

func (s *Steps) GivenActiveTrip(startedAt time.Time) domaintrip.Trip {
	s.t.Helper()

	value, err := domaintrip.NewTrip(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		55.75,
		37.61,
		55.76,
		37.62,
		500,
		startedAt.UTC().Truncate(time.Microsecond),
	)
	require.NoError(s.t, err)
	require.NoError(s.t, s.app.TripRepository.Create(s.t.Context(), *value))
	return *value
}

func (s *Steps) GivenCompletedTrip(startedAt time.Time) domaintrip.Trip {
	s.t.Helper()

	value := s.GivenActiveTrip(startedAt)
	require.NoError(s.t, value.Finish(startedAt.Add(time.Hour)))
	require.NoError(s.t, s.app.TripRepository.Update(s.t.Context(), value))
	return value
}

func (s *Steps) GivenLastPositionAt(tripID uuid.UUID, recordedAt time.Time) {
	s.t.Helper()
	require.NoError(
		s.t,
		s.app.TripRepository.UpdateLastPositionAt(s.t.Context(), tripID, recordedAt),
	)
}

func (s *Steps) WhenPositionIsSaved(
	tripID uuid.UUID,
	recordedAt time.Time,
) domainposition.Position {
	s.t.Helper()

	created, err := s.app.CreateService.Create(s.t.Context(), positioncreate.Input{
		TripID:     tripID,
		Latitude:   59.935,
		Longitude:  30.335,
		RecordedAt: recordedAt,
	})
	require.NoError(s.t, err)
	return created
}

func (s *Steps) WhenStaleTripsAreSelected(
	staleAfter time.Duration,
	batchSize int32,
	ptr *positions.FetchForPushPtr,
) positions.FetchForPushResult {
	s.t.Helper()

	result, err := s.app.TripRepository.FetchForPushQuery(
		s.t.Context(),
		staleAfter,
		batchSize,
		ptr,
	)
	require.NoError(s.t, err)
	return result
}

func (s *Steps) ThenOnlyTripsAreSelected(
	result positions.FetchForPushResult,
	expected ...uuid.UUID,
) {
	s.t.Helper()

	actual := make([]uuid.UUID, len(result.SelectedTrips))
	for i, trip := range result.SelectedTrips {
		actual[i] = trip.TripID
	}
	assert.ElementsMatch(s.t, expected, actual)
}

func (s *Steps) ThenLastPositionAtEquals(tripID uuid.UUID, expected time.Time) {
	s.t.Helper()

	var actual *time.Time
	err := s.app.Pool.QueryRow(
		s.t.Context(),
		"SELECT last_position_at FROM trips WHERE id = $1",
		tripID,
	).Scan(&actual)
	require.NoError(s.t, err)
	require.NotNil(s.t, actual)
	assert.True(s.t, expected.Equal(*actual))
}

func (s *Steps) ThenLastPositionAtIsNull(tripID uuid.UUID) {
	s.t.Helper()

	var actual *time.Time
	err := s.app.Pool.QueryRow(
		s.t.Context(),
		"SELECT last_position_at FROM trips WHERE id = $1",
		tripID,
	).Scan(&actual)
	require.NoError(s.t, err)
	assert.Nil(s.t, actual)
}

func (s *Steps) ThenRouteContainsPositions(
	tripID uuid.UUID,
	expected ...domainposition.Position,
) {
	s.t.Helper()

	actual, err := s.app.PositionRepository.GetListByTripID(s.t.Context(), tripID)
	require.NoError(s.t, err)
	require.Len(s.t, actual, len(expected))
	for i := range expected {
		assert.Equal(s.t, expected[i].ID, actual[i].ID)
	}
}

func (s *Steps) ThenNoPositionsAreStored(tripID uuid.UUID) {
	s.t.Helper()

	var count int
	err := s.app.Pool.QueryRow(
		s.t.Context(),
		"SELECT count(*) FROM trip_positions WHERE trip_id = $1",
		tripID,
	).Scan(&count)
	require.NoError(s.t, err)
	assert.Zero(s.t, count)
}
