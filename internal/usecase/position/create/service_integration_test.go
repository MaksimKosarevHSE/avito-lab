//go:build integration

package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	triprepo "github.com/MaksimKosarevHSE/avito-lab/internal/repository/postgres/trip"
	positiontest "github.com/MaksimKosarevHSE/avito-lab/internal/testkit/position"
	"github.com/MaksimKosarevHSE/avito-lab/internal/usecase/position/create"
)

type ServiceSuite struct {
	suite.Suite

	app   *positiontest.App
	steps *positiontest.Steps
}

func (s *ServiceSuite) SetupSuite() {
	s.app = positiontest.NewApp(s.T())
}

func (s *ServiceSuite) SetupTest() {
	s.app.ResetTrips(s.T())
	s.steps = positiontest.NewSteps(s.T(), s.app)
}

func (s *ServiceSuite) TestSavingPositionUpdatesLastPositionAtWithoutMovingItBackwards() {
	trip := s.steps.GivenActiveTrip(time.Now().Add(-time.Hour))
	newerTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	olderTime := newerTime.Add(-time.Minute)

	newer := s.steps.WhenPositionIsSaved(trip.ID, newerTime)
	older := s.steps.WhenPositionIsSaved(trip.ID, olderTime)

	s.steps.ThenLastPositionAtEquals(trip.ID, newerTime)
	s.steps.ThenRouteContainsPositions(trip.ID, older, newer)
}

func (s *ServiceSuite) TestSavingPositionRollsBackWhenTripUpdateFails() {
	trip := s.steps.GivenActiveTrip(time.Now().Add(-time.Hour))
	updateErr := errors.New("update last position failed")
	service := create.NewService(
		s.app.PositionRepository,
		failingTripRepository{Repository: s.app.TripRepository, err: updateErr},
		s.app.TxManager,
		noopCanceller{},
		noopMetrics{},
	)

	_, err := service.Create(s.T().Context(), create.Input{
		TripID:     trip.ID,
		Latitude:   59.935,
		Longitude:  30.335,
		RecordedAt: time.Now().UTC(),
	})

	require.ErrorIs(s.T(), err, updateErr)
	s.steps.ThenNoPositionsAreStored(trip.ID)
	s.steps.ThenLastPositionAtIsNull(trip.ID)
}

type failingTripRepository struct {
	*triprepo.Repository
	err error
}

func (r failingTripRepository) UpdateLastPositionAt(context.Context, uuid.UUID, time.Time) error {
	return r.err
}

type noopCanceller struct{}

func (noopCanceller) Cancel(uuid.UUID) bool {
	return false
}

type noopMetrics struct{}

func (noopMetrics) AddPositionSaved(context.Context) {}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceSuite))
}
