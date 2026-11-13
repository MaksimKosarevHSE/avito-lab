package create_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
)

type createMetricsSpy struct {
	created int
}

func (m *createMetricsSpy) TripCreated(context.Context) {
	m.created++
}

func TestService_TripCreatedMetricFollowsTransactionResult(t *testing.T) {
	transactionError := errors.New("commit failed")
	tests := []struct {
		name      string
		doError   error
		wantCalls int
	}{
		{name: "committed", wantCalls: 1},
		{name: "commit failed", doError: transactionError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMockTripRepository(ctrl)
			txManager := NewMockTxManager(ctrl)
			metrics := &createMetricsSpy{}
			service := create.NewService(
				txManager,
				repository,
				time.Hour,
				create.WithObservability(slog.New(slog.DiscardHandler), metrics),
			)

			txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, fn func(context.Context) error) error {
					require.NoError(t, fn(ctx))
					require.Zero(t, metrics.created)
					return test.doError
				},
			)
			repository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			repository.EXPECT().AddStatusHistory(gomock.Any(), gomock.Any()).Return(nil)

			_, err := service.Create(context.Background(), validCreateInput())
			if test.doError != nil {
				require.ErrorIs(t, err, test.doError)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantCalls, metrics.created)
		})
	}
}

func TestService_IdempotencyReplayDoesNotIncrementCreatedMetric(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockTripRepository(ctrl)
	txManager := NewMockTxManager(ctrl)
	metrics := &createMetricsSpy{}
	service := create.NewService(
		txManager,
		repository,
		time.Hour,
		create.WithObservability(slog.New(slog.DiscardHandler), metrics),
	)
	input := validCreateInput()
	idempotencyKey := uuid.New()
	input.IdempotencyKey = &idempotencyKey

	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)
	var claimed domaintrip.IdempotencyRecord
	repository.EXPECT().ClaimIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			record domaintrip.IdempotencyRecord,
			_ time.Duration,
		) (bool, error) {
			claimed = record
			return false, nil
		})
	repository.EXPECT().GetIdempotencyKey(gomock.Any(), idempotencyKey).
		DoAndReturn(func(context.Context, uuid.UUID) (domaintrip.IdempotencyRecord, error) {
			return claimed, nil
		})
	repository.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(domaintrip.Trip{ID: uuid.New()}, nil)

	result, err := service.Create(context.Background(), input)
	require.NoError(t, err)
	require.True(t, result.Replayed)
	require.Zero(t, metrics.created)
}

func validCreateInput() create.CreateInput {
	return create.CreateInput{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55,
		StartLongitude: 37,
		EndLatitude:    56,
		EndLongitude:   38,
		Price:          200,
	}
}
