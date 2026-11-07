//go:build integration

package trip_test

import (
	"context"
	"testing"
	"time"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/testkit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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

func (s *RepositorySuite) TestInsertsInTwoTablesInOneTransactionAreRollbacks() {
	t := s.T()

	err := s.postgres.TxManager.Do(t.Context(), func(ctx context.Context) error {
		err := s.repository.Create(ctx, *s.tripFixture)
		require.NoError(t, err)

		_, err = s.repository.GetByID(ctx, s.tripFixture.ID)
		require.NoError(t, err)
		return s.repository.AddStatusHistory(ctx, domaintrip.StatusHistory{})
	})
	require.Error(t, err)

	_, err = s.repository.GetByID(t.Context(), s.tripFixture.ID)
	require.ErrorIs(t, err, domaintrip.ErrTripNotFound)
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
func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(RepositorySuite))
}
