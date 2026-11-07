package finish_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	trip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/finish"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

type fixture struct {
	Ctx    context.Context
	TripID uuid.UUID

	Handler *finish.Handler
	Service *MocktripFinisher
}

func NewFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	service := NewMocktripFinisher(ctrl)
	handler := finish.New(service)

	return &fixture{
		Ctx:     context.Background(),
		TripID:  uuid.New(),
		Handler: handler,
		Service: service,
	}
}

func TestHandler_FinishTrip_Success(t *testing.T) {
	f := NewFixture(t)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/"+f.TripID.String()+"/finish",
		nil)

	startedAt := time.Now()
	finishedAt := startedAt.Add(time.Hour)

	completedTrip := trip.Trip{
		ID:             f.TripID,
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55,
		StartLongitude: 37,
		EndLatitude:    56,
		EndLongitude:   38,
		Price:          200,
		Status:         trip.StatusCompleted,
		StartedAt:      startedAt,
		FinishedAt:     &finishedAt,
		CreatedAt:      startedAt,
		UpdatedAt:      finishedAt,
	}

	f.Service.EXPECT().
		Finish(gomock.Any(), f.TripID).
		Return(completedTrip, nil)

	recorder := httptest.NewRecorder()

	f.Handler.FinishTrip(recorder, request, f.TripID)
	assert.Equal(
		t,
		"application/json",
		recorder.Header().Get("Content-Type"),
	)

	var response api.Trip
	err := json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)

	assert.Equal(t, completedTrip.ID, response.Id)
	assert.Equal(t, completedTrip.UserID, response.UserId)
	assert.Equal(t, completedTrip.DriverID, response.DriverId)
	assert.Equal(t, api.Completed, response.Status)
	assert.Equal(t, completedTrip.Price, response.Price)
	require.NotNil(t, response.FinishedAt)
	assert.True(t, finishedAt.Equal(*response.FinishedAt))

}

func TestHandler_TryFinishAlreadyFinishedTrip(t *testing.T) {
	f := NewFixture(t)
	now := time.Now()
	finishedTrip := trip.Trip{
		ID:             f.TripID,
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  24.5,
		StartLongitude: 24.6,
		EndLatitude:    27.5,
		EndLongitude:   27.6,
		Price:          200,
		Status:         trip.StatusActive,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	err := finishedTrip.Finish(time.Now())
	require.NoError(t, err)

	f.Service.EXPECT().
		Finish(gomock.Any(), gomock.Any()).
		Return(trip.Trip{}, trip.ErrTripCompleted)

	request := httptest.NewRequest(http.MethodPost, "/api/trips/"+f.TripID.String()+"/finish", nil)
	recorder := httptest.NewRecorder()
	f.Handler.FinishTrip(recorder, request, f.TripID)
	var response api.Problem
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))

	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Equal(t, "trip_completed", response.Code)
	assert.Equal(t, int32(http.StatusConflict), response.Status)
}
