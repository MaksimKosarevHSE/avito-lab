package create_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
)

type fixture struct {
	Ctx      context.Context
	TripID   uuid.UUID
	DriverID uuid.UUID
	UserID   uuid.UUID

	Repository *MockTripRepository
	Service    *create.Service
	TxManager  *MockTxManager
}

func NewFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	repository := NewMockTripRepository(ctrl)
	txManager := NewMockTxManager(ctrl)

	service := create.NewService(txManager, repository, time.Hour)

	return &fixture{
		Ctx:        context.Background(),
		TripID:     uuid.New(),
		DriverID:   uuid.New(),
		UserID:     uuid.New(),
		Repository: repository,
		Service:    service,
		TxManager:  txManager,
	}
}

func TestService_CreateTripWithoutClaimNewIdempotencyKey(t *testing.T) {
	f := NewFixture(t)

	idempotencyKey := uuid.New()

	input := create.CreateInput{
		UserID:         f.UserID,
		DriverID:       f.DriverID,
		StartLatitude:  1,
		StartLongitude: 1,
		EndLatitude:    1,
		EndLongitude:   1,
		Price:          200,
		IdempotencyKey: &idempotencyKey,
	}

	var (
		grappedNowTime time.Time
		grappedID      uuid.UUID
	)

	f.TxManager.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context,
			fn func(ctx context.Context) error,
		) error {
			return fn(ctx)
		})

	f.Repository.EXPECT().
		ClaimIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, nil)

	f.Repository.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, trip trip.Trip) error {
			grappedID = trip.ID
			grappedNowTime = trip.CreatedAt
			return nil
		}).
		Return(nil)

	f.Repository.EXPECT().
		AddStatusHistory(f.Ctx, gomock.Any()).
		Return(nil)

	got, err := f.Service.Create(f.Ctx, input)

	want := create.CreateResult{
		Trip: trip.Trip{
			ID:             grappedID,
			UserID:         f.UserID,
			DriverID:       f.DriverID,
			StartLatitude:  1,
			StartLongitude: 1,
			EndLatitude:    1,
			EndLongitude:   1,
			Price:          200,
			Status:         trip.StatusActive,
			StartedAt:      grappedNowTime,
			FinishedAt:     nil,
			CreatedAt:      grappedNowTime,
			UpdatedAt:      grappedNowTime,
		},
		Replayed: false,
	}

	require.NoError(t, err)
	require.Equal(t, want, got)
}
