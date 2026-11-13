package finish_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/finish"
)

type finishMetricsSpy struct {
	completed int
}

func (m *finishMetricsSpy) TripCompleted(context.Context) {
	m.completed++
}

func TestService_TripCompletedMetricFollowsTransactionResult(t *testing.T) {
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
			metrics := &finishMetricsSpy{}
			service := finish.NewService(
				txManager,
				repository,
				finish.WithObservability(slog.New(slog.DiscardHandler), metrics),
			)
			activeTrip := activeTripFixture()

			txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, fn func(context.Context) error) error {
					require.NoError(t, fn(ctx))
					require.Zero(t, metrics.completed)
					return test.doError
				},
			)
			repository.EXPECT().GetByIDForUpdate(gomock.Any(), activeTrip.ID).
				Return(activeTrip, nil)
			repository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			repository.EXPECT().AddStatusHistory(gomock.Any(), gomock.Any()).Return(nil)

			_, err := service.Finish(context.Background(), activeTrip.ID)
			if test.doError != nil {
				require.ErrorIs(t, err, test.doError)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantCalls, metrics.completed)
		})
	}
}

func activeTripFixture() trip.Trip {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return trip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55,
		StartLongitude: 37,
		EndLatitude:    56,
		EndLongitude:   38,
		Price:          200,
		Status:         trip.StatusActive,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
