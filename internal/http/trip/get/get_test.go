package get_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	httpget "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/get"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fixture struct {
	Ctx    context.Context
	TripID uuid.UUID

	Handler *httpget.Handler
	Service *MocktripGetter
}

func NewFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	service := NewMocktripGetter(ctrl)
	handler := httpget.New(service)

	return &fixture{
		Ctx:     context.Background(),
		TripID:  uuid.New(),
		Handler: handler,
		Service: service,
	}
}

func TestHandler_GetTrip_Success(t *testing.T) {
	f := NewFixture(t)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/trips/"+f.TripID.String(),
		nil,
	)

	startedAt := time.Now()
	activeTrip := trip.Trip{
		ID:             f.TripID,
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55,
		StartLongitude: 37,
		EndLatitude:    56,
		EndLongitude:   38,
		Price:          200,
		Status:         trip.StatusActive,
		StartedAt:      startedAt,
		CreatedAt:      startedAt,
		UpdatedAt:      startedAt,
	}

	f.Service.EXPECT().
		Get(gomock.Any(), f.TripID).
		Return(activeTrip, nil)

	recorder := httptest.NewRecorder()

	f.Handler.GetTrip(recorder, request, f.TripID)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response api.Trip
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))

	assert.Equal(t, activeTrip.ID, response.Id)
	assert.Equal(t, activeTrip.UserID, response.UserId)
	assert.Equal(t, activeTrip.DriverID, response.DriverId)
	assert.Equal(t, activeTrip.StartLatitude, response.StartPoint.Latitude)
	assert.Equal(t, activeTrip.StartLongitude, response.StartPoint.Longitude)
	assert.Equal(t, activeTrip.EndLatitude, response.EndPoint.Latitude)
	assert.Equal(t, activeTrip.EndLongitude, response.EndPoint.Longitude)
	assert.Equal(t, activeTrip.Price, response.Price)
	assert.Equal(t, api.Active, response.Status)
	assert.True(t, startedAt.Equal(response.StartedAt))
	assert.Nil(t, response.FinishedAt)
}

func TestHandler_GetTrip_NotFound(t *testing.T) {
	f := NewFixture(t)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/trips/"+f.TripID.String(),
		nil,
	)

	f.Service.EXPECT().
		Get(gomock.Any(), f.TripID).
		Return(trip.Trip{}, trip.ErrTripNotFound)

	recorder := httptest.NewRecorder()

	f.Handler.GetTrip(recorder, request, f.TripID)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "application/problem+json", recorder.Header().Get("Content-Type"))

	var problem api.Problem
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&problem))

	assert.Equal(t, int32(http.StatusNotFound), problem.Status)
	assert.Equal(t, "trip_not_found", problem.Code)
	assert.Equal(t, "Trip not found", problem.Title)
	require.NotNil(t, problem.Detail)
	assert.Equal(t, "Trip was not found", *problem.Detail)
}

func TestHadler_DoNotShowInternalAndRepositoryErrorsDetails(t *testing.T) {
	f := NewFixture(t)
	internalError := errors.New("INTERNAL ERROR TEXT")

	f.Service.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(trip.Trip{}, internalError)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+f.TripID.String(), nil)
	recorder := httptest.NewRecorder()
	f.Handler.GetTrip(recorder, request, f.TripID)

	body := recorder.Body.Bytes()
	var problem api.Problem
	require.NoError(t, json.Unmarshal(body, &problem))

	assert.Equal(t, int32(http.StatusInternalServerError), problem.Status)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, "internal_error", problem.Code)
	require.NotContains(t, string(body), internalError.Error())
	require.NotNil(t, problem.Detail)
	assert.Equal(t, "Internal server error", *problem.Detail)
}
