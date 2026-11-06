package trip_test

import (
	"testing"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTrip_Validation(t *testing.T) {
	validID := uuid.New()
	validDriverID := uuid.New()
	validUserID := uuid.New()
	now := time.Now()

	tests := []struct {
		name           string
		wantErr        error
		id             uuid.UUID
		userID         uuid.UUID
		driverID       uuid.UUID
		startLatitude  float64
		startLongitude float64
		endLatitude    float64
		endLongitude   float64
		price          int64
		createdAt      time.Time
	}{
		{
			name:           "valid trip 1",
			wantErr:        nil,
			id:             validID,
			userID:         validUserID,
			driverID:       validDriverID,
			startLatitude:  -10.3,
			startLongitude: 52.2,
			endLatitude:    54.4,
			endLongitude:   -80.3,
			price:          1000,
			createdAt:      now,
		},
		{
			name:           "valid trip 2",
			wantErr:        nil,
			id:             validID,
			userID:         validUserID,
			driverID:       validDriverID,
			startLatitude:  -10,
			startLongitude: 52,
			endLatitude:    54,
			endLongitude:   -80,
			price:          1,
			createdAt:      now,
		},
		{
			name:           "missing trip ID",
			wantErr:        trip.ErrInvalidTrip,
			userID:         validUserID,
			driverID:       validDriverID,
			startLatitude:  1,
			startLongitude: 1,
			endLatitude:    1,
			endLongitude:   1,
			price:          1,
			createdAt:      now,
		},
		{
			name:           "missing driver ID",
			wantErr:        trip.ErrInvalidTrip,
			id:             validID,
			userID:         validUserID,
			startLatitude:  1,
			startLongitude: 1,
			endLatitude:    1,
			endLongitude:   1,
			price:          1,
			createdAt:      now,
		},
		{
			name:           "missing user ID",
			wantErr:        trip.ErrInvalidTrip,
			driverID:       validDriverID,
			id:             validID,
			startLatitude:  1,
			startLongitude: 1,
			endLatitude:    1,
			endLongitude:   1,
			price:          1,
			createdAt:      now,
		},
		{
			name:           "missing create time",
			wantErr:        trip.ErrInvalidTrip,
			id:             validID,
			userID:         validUserID,
			driverID:       validDriverID,
			startLatitude:  1,
			startLongitude: 1,
			endLatitude:    1,
			endLongitude:   1,
			price:          1,
		},
		{
			name:           "negative price",
			wantErr:        trip.ErrInvalidTrip,
			id:             validID,
			userID:         validUserID,
			driverID:       validDriverID,
			startLatitude:  1,
			startLongitude: 1,
			endLatitude:    1,
			endLongitude:   1,
			price:          -1,
			createdAt:      now,
		},
		{
			name:           "invalid coordinates",
			wantErr:        trip.ErrInvalidTrip,
			id:             validID,
			userID:         validUserID,
			driverID:       validDriverID,
			startLatitude:  -181,
			startLongitude: -200,
			endLatitude:    2134,
			endLongitude:   200,
			price:          143,
			createdAt:      now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := trip.NewTrip(
				tt.id,
				tt.userID,
				tt.driverID,
				tt.startLatitude,
				tt.startLongitude,
				tt.endLatitude,
				tt.endLongitude,
				tt.price,
				tt.createdAt,
			)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			expected := &trip.Trip{
				ID:             tt.id,
				UserID:         tt.userID,
				DriverID:       tt.driverID,
				StartLatitude:  tt.startLatitude,
				StartLongitude: tt.startLongitude,
				EndLatitude:    tt.endLatitude,
				EndLongitude:   tt.endLongitude,
				Price:          tt.price,
				Status:         trip.StatusActive,
				StartedAt:      tt.createdAt,
				FinishedAt:     nil,
				CreatedAt:      tt.createdAt,
				UpdatedAt:      tt.createdAt,
			}
			require.NoError(t, err)
			require.NotNil(t, value)
			assert.Equal(t, expected, value)
		})
	}
}

func TestNewTrip_CreatesActiveTrip(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	tm := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := trip.NewTrip(
		id,
		userID,
		driverID,
		10,
		10.5,
		10,
		10,
		200,
		tm,
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	expected := &trip.Trip{
		ID:             id,
		UserID:         userID,
		DriverID:       driverID,
		StartLatitude:  10,
		StartLongitude: 10.5,
		EndLatitude:    10,
		EndLongitude:   10,
		Price:          200,
		Status:         trip.StatusActive,
		StartedAt:      tm,
		FinishedAt:     nil,
		CreatedAt:      tm,
		UpdatedAt:      tm,
	}

	assert.Equal(t, expected, result)
}

func TestFinish_SuccessfullyFinish(t *testing.T) {
	tmStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tmEnd := tmStart.Add(time.Hour)

	testTrip := &trip.Trip{
		Status:    trip.StatusActive,
		StartedAt: tmStart,
		UpdatedAt: tmStart,
	}

	err := testTrip.Finish(tmEnd)

	require.NoError(t, err)
	assert.Equal(t, trip.StatusCompleted, testTrip.Status)
	assert.Equal(t, tmStart, testTrip.StartedAt)
	require.NotNil(t, testTrip.FinishedAt)
	assert.Equal(t, tmEnd, *testTrip.FinishedAt)
	assert.Equal(t, tmEnd, testTrip.UpdatedAt)
}

func TestFinish_TripAlreadyCompleted(t *testing.T) {
	stTime := time.Now()
	fTime := stTime.Add(time.Hour)

	testTrip := &trip.Trip{
		Status:     trip.StatusCompleted,
		StartedAt:  stTime,
		UpdatedAt:  fTime,
		FinishedAt: &fTime,
	}
	beforeFinish := *testTrip
	err := testTrip.Finish(fTime.Add(time.Minute))
	assert.ErrorIs(t, err, trip.ErrTripCompleted)
	assert.Equal(t, beforeFinish, *testTrip)
}
