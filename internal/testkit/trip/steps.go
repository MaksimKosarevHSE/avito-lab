//go:build integration

package triptest

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	tripcreate "github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
)

type Steps struct {
	t   testing.TB
	app *App
}

func NewSteps(t testing.TB, app *App) *Steps {
	t.Helper()
	require.NotNil(t, app)

	return &Steps{
		t:   t,
		app: app,
	}
}

func (s *Steps) GivenUserID() uuid.UUID {
	s.t.Helper()
	return uuid.New()
}

func (s *Steps) GivenDriverID() uuid.UUID {
	s.t.Helper()
	return uuid.New()
}

func (s *Steps) WhenTripIsCreated(userID, driverID uuid.UUID) domaintrip.Trip {
	s.t.Helper()

	result, err := s.app.CreateService.Create(s.t.Context(), tripcreate.CreateInput{
		UserID:         userID,
		DriverID:       driverID,
		StartLatitude:  55.751244,
		StartLongitude: 37.618423,
		EndLatitude:    55.760186,
		EndLongitude:   37.618711,
		Price:          500,
	})
	require.NoError(s.t, err)
	require.False(s.t, result.Replayed)
	require.Equal(s.t, domaintrip.StatusActive, result.Trip.Status)

	return result.Trip
}

func (s *Steps) WhenTripIsFinished(tripID uuid.UUID) domaintrip.Trip {
	s.t.Helper()

	finished, err := s.app.FinishService.Finish(s.t.Context(), tripID)
	require.NoError(s.t, err)
	require.Equal(s.t, domaintrip.StatusCompleted, finished.Status)
	require.NotNil(s.t, finished.FinishedAt)

	return finished
}
