//go:build integration

package trip_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/testkit"
)

type RepositorySuite struct {
	suite.Suite

	postgres   *testkit.Postgres
	repository *trip.Repository

	tripFixture *domaintrip.Trip
}

func (s *RepositorySuite) SetupSuite() {
	s.postgres = testkit.NewPostgres(s.T())
	s.repository = trip.New(s.postgres.Database)
}

func (s *RepositorySuite) SetupTest() {
	s.postgres.ResetTrips(s.T())

	tripFixture, err := domaintrip.NewTrip(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		54.5,
		55.5,
		56.5,
		57.6,
		200,
		time.Now().UTC().Truncate(time.Microsecond),
	)
	require.NoError(s.T(), err)
	s.tripFixture = tripFixture
}

func (s *RepositorySuite) TestCreateAndGetSuccess() {
	err := s.repository.Create(s.T().Context(), *s.tripFixture)
	require.NoError(s.T(), err)

	got, err := s.repository.GetByID(s.T().Context(), s.tripFixture.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), *s.tripFixture, got)
}

func (s *RepositorySuite) TestUpdateTrip() {
	expected := *s.tripFixture

	require.NoError(
		s.T(), s.repository.Create(s.T().Context(), *s.tripFixture),
	)

	finishedAt := time.Now().UTC().Truncate(time.Microsecond)
	err := expected.Finish(finishedAt)
	require.NoError(s.T(), err)

	err = s.repository.Update(s.T().Context(), expected)
	require.NoError(s.T(), err)

	got, err := s.repository.GetByID(s.T().Context(), expected.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), expected, got)
}

func (s *RepositorySuite) TestInsertsInTwoTablesInOneTransactionAreRolledBack() {
	const countRowsQuery = `SELECT
		(SELECT count(*) FROM trips WHERE id = $1),
		(SELECT count(*) FROM trip_status_history WHERE trip_id = $1)`

	t := s.T()
	rollbackErr := errors.New("transaction rollback")
	reason := "trip created"

	err := s.postgres.TxManager.Do(t.Context(), func(ctx context.Context) error {
		err := s.repository.Create(ctx, *s.tripFixture)
		require.NoError(t, err)

		err = s.repository.AddStatusHistory(ctx, domaintrip.StatusHistory{
			TripID:    s.tripFixture.ID,
			ToStatus:  domaintrip.StatusActive,
			Reason:    &reason,
			ChangedAt: s.tripFixture.CreatedAt,
		})
		require.NoError(t, err)

		tx, ok := s.postgres.TxManager.Transaction(ctx)
		require.True(t, ok)

		var tripCount int64
		var historyCount int64
		err = tx.QueryRow(
			ctx,
			countRowsQuery,
			s.tripFixture.ID,
		).Scan(&tripCount, &historyCount)
		require.NoError(t, err)
		require.EqualValues(t, 1, tripCount)
		require.EqualValues(t, 1, historyCount)

		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	var tripCount int64
	var historyCount int64
	err = s.postgres.Pool.QueryRow(
		t.Context(),
		countRowsQuery,
		s.tripFixture.ID,
	).Scan(&tripCount, &historyCount)
	require.NoError(t, err)
	assert.Zero(t, tripCount)
	assert.Zero(t, historyCount)
}

func (s *RepositorySuite) TestCreateReturnsDriverBusy() {
	t := s.T()
	trip1 := *s.tripFixture

	trip2 := *s.tripFixture
	trip2.ID = uuid.New()

	err := s.repository.Create(t.Context(), trip1)
	require.NoError(t, err)
	err = s.repository.Create(t.Context(), trip2)
	require.ErrorIs(t, err, domaintrip.ErrDriverBusy)
}

func (s *RepositorySuite) TestCountActive() {
	t := s.T()

	count, err := s.repository.CountActive(t.Context())
	require.NoError(t, err)
	assert.Zero(t, count)

	require.NoError(t, s.repository.Create(t.Context(), *s.tripFixture))

	count, err = s.repository.CountActive(t.Context())
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	completed := *s.tripFixture
	require.NoError(t, completed.Finish(time.Now().UTC().Truncate(time.Microsecond)))
	require.NoError(t, s.repository.Update(t.Context(), completed))

	count, err = s.repository.CountActive(t.Context())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(RepositorySuite))
}
