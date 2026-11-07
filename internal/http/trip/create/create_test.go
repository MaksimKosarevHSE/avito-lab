package create_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	httpcreate "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/create"
	create "github.com/MaksimKosarevHSE/avito-lab/internal/service/trip/create"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

type fixture struct {
	Ctx      context.Context
	TripID   uuid.UUID
	DriverID uuid.UUID
	UserID   uuid.UUID

	Handler *httpcreate.Handler
	Service *MocktripCreator
}

func NewFixture(t *testing.T) *fixture {
	t.Helper()

	ctrl := gomock.NewController(t)
	service := NewMocktripCreator(ctrl)
	handler := httpcreate.New(service)

	return &fixture{
		Ctx:      context.Background(),
		TripID:   uuid.New(),
		DriverID: uuid.New(),
		UserID:   uuid.New(),
		Handler:  handler,
		Service:  service,
	}
}

func TestHandler_CreateTrip_Success(t *testing.T) {
	f := NewFixture(t)
	requestBody := api.CreateTripJSONRequestBody{
		UserId:   f.UserID,
		DriverId: f.DriverID,
		Price:    200,
		StartPoint: api.Coordinates{
			Latitude:  24.5,
			Longitude: 24.6,
		},
		EndPoint: api.Coordinates{
			Latitude:  27.5,
			Longitude: 27.6,
		},
	}
	payload, err := json.Marshal(requestBody)
	require.NoError(t, err)

	now := time.Now()
	idempotencyKey := uuid.New()

	createdTrip := trip.Trip{
		ID:             f.TripID,
		UserID:         f.UserID,
		DriverID:       f.DriverID,
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

	f.Service.EXPECT().
		Create(gomock.Any(), create.CreateInput{
			UserID:         f.UserID,
			DriverID:       f.DriverID,
			StartLatitude:  24.5,
			StartLongitude: 24.6,
			EndLatitude:    27.5,
			EndLongitude:   27.6,
			Price:          200,
			IdempotencyKey: &idempotencyKey,
		}).
		Return(create.CreateResult{
			Trip:     createdTrip,
			Replayed: false,
		}, nil)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips",
		bytes.NewReader(payload),
	)

	request.Header.Set("Content-Type:", "application/json")
	recorder := httptest.NewRecorder()

	f.Handler.CreateTrip(recorder, request, api.CreateTripParams{
		IdempotencyKey: &idempotencyKey,
	})

	require.Equal(t, http.StatusCreated, recorder.Code)

	var response api.Trip
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))

	assert.Equal(t, createdTrip.ID, response.Id)
	assert.Equal(t, createdTrip.DriverID, response.DriverId)
	assert.Equal(t, createdTrip.UserID, response.UserId)
	assert.Equal(t, api.TripStatus(trip.StatusActive), response.Status)
}
