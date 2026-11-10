package domainerror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

func TestProblemMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
		wantTitle  string
		wantCode   string
		wantDetail string
	}{
		{
			name:       "invalid trip",
			err:        fmt.Errorf("%w: price must not be negative", domaintrip.ErrInvalidTrip),
			wantStatus: http.StatusBadRequest,
			wantType:   "https://tripgo.example/problems/invalid-request",
			wantTitle:  "Invalid request",
			wantCode:   "invalid_request",
			wantDetail: "invalid trip: price must not be negative",
		},
		{
			name:       "trip not found",
			err:        fmt.Errorf("get trip: %w", domaintrip.ErrTripNotFound),
			wantStatus: http.StatusNotFound,
			wantType:   "https://tripgo.example/problems/trip-not-found",
			wantTitle:  "Trip not found",
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "trip already completed",
			err:        domaintrip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantType:   "https://tripgo.example/problems/trip-completed",
			wantTitle:  "Trip completed",
			wantCode:   "trip_completed",
			wantDetail: "Operation is not allowed for a completed trip",
		},
		{
			name:       "driver busy",
			err:        domaintrip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantType:   "https://tripgo.example/problems/driver-busy",
			wantTitle:  "Driver busy",
			wantCode:   "driver_busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name:       "idempotency conflict",
			err:        domaintrip.ErrIdempotencyConflict,
			wantStatus: http.StatusConflict,
			wantType:   "https://tripgo.example/problems/idempotency-conflict",
			wantTitle:  "Idempotency conflict",
			wantCode:   "idempotency_conflict",
			wantDetail: "Idempotency-Key was already used with a different request body",
		},
		{
			name:       "unknown internal error",
			err:        errors.New("database password is secret"),
			wantStatus: http.StatusInternalServerError,
			wantType:   "https://tripgo.example/problems/internal-error",
			wantTitle:  "Internal Server Error",
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, problem := domainerror.Problem(tt.err)

			assert.Equal(t, tt.wantStatus, status)
			assert.EqualValues(t, tt.wantStatus, problem.Status)
			assert.Equal(t, tt.wantCode, problem.Code)
			assert.Equal(t, tt.wantType, problem.Type)
			assert.Equal(t, tt.wantTitle, problem.Title)

			require.NotNil(t, problem.Detail)
			assert.Equal(t, tt.wantDetail, *problem.Detail)
		})
	}
}
