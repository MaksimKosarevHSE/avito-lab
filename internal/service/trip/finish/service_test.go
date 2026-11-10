package finish_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/finish"
)

type fixture struct {
	Ctx  context.Context
	Trip trip.Trip

	Repository *MockTripRepository
	TxManager  *MockTxManager
	Service    *finish.Service
}

func NewFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	repository := NewMockTripRepository(ctrl)
	txManager := NewMockTxManager(ctrl)
	service := finish.NewService(txManager, repository)

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
		TxManager:  txManager,
		Service:    service,
	}
}

func TestService_FinishSuccessfully(t *testing.T) {
	f := NewFixture(t)

	f.TxManager.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(_ context.Context) error) error {
			return fn(ctx)
		})

	f.Repository.EXPECT().
		GetByIDForUpdate(gomock.Any(), gomock.Any()).
		Return(f.Trip, nil)

	f.Repository.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	f.Repository.EXPECT().
		AddStatusHistory(gomock.Any(), gomock.Any()).
		Return(nil)

	got, err := f.Service.Finish(f.Ctx, f.Trip.ID)

	require.NoError(t, err)

	assert.Equal(t, f.Trip.ID, got.ID)
	assert.Equal(t, f.Trip.UserID, got.UserID)
	assert.Equal(t, f.Trip.DriverID, got.DriverID)
	assert.Equal(t, trip.StatusCompleted, got.Status)

	assert.NotNil(t, got.FinishedAt)
	assert.Equal(t, *got.FinishedAt, got.UpdatedAt)
}

func TestService_DoNotFinishAlreadyFinished(t *testing.T) {
	f := NewFixture(t)
	now := time.Now()

	finishedTrip := f.Trip
	require.NoError(t, finishedTrip.Finish(now))

	f.TxManager.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(_ context.Context) error) error {
			return fn(ctx)
		})

	f.Repository.EXPECT().
		GetByIDForUpdate(gomock.Any(), gomock.Any()).
		Return(finishedTrip, nil)

	got, err := f.Service.Finish(f.Ctx, f.Trip.ID)

	assert.ErrorIs(t, err, trip.ErrTripCompleted)
	assert.Equal(t, trip.Trip{}, got)
}
