package get_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/get"
)

type fixture struct {
	Ctx  context.Context
	Trip trip.Trip

	Repository *MockTripRepository
	Service    *get.Service
}

func NewFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	repository := NewMockTripRepository(ctrl)
	service := get.NewService(repository)

	time := time.Now()
	trip := trip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  1,
		StartLongitude: 1,
		EndLatitude:    1,
		EndLongitude:   1,
		Price:          200,
		Status:         trip.StatusActive,
		StartedAt:      time,
		FinishedAt:     nil,
		CreatedAt:      time,
		UpdatedAt:      time,
	}

	return &fixture{
		Ctx:        context.Background(),
		Trip:       trip,
		Repository: repository,
		Service:    service,
	}
}

func TestService_GetTripSuccess(t *testing.T) {
	f := NewFixture(t)

	f.Repository.EXPECT().
		GetByID(gomock.Any(), f.Trip.ID).
		Return(f.Trip, nil)

	got, err := f.Service.Get(f.Ctx, f.Trip.ID)
	require.NoError(t, err)
	assert.Equal(t, f.Trip, got)
}

func TestService_GetTripDoNotExists(t *testing.T) {
	f := NewFixture(t)

	f.Repository.EXPECT().
		GetByID(gomock.Any(), f.Trip.ID).
		Return(trip.Trip{}, trip.ErrTripNotFound)

	_, err := f.Service.Get(f.Ctx, f.Trip.ID)
	require.ErrorIs(t, err, trip.ErrTripNotFound)
}
