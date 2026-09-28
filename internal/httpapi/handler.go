package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/trip"
)

const maxCreateTripBodyBytes = 1 << 20

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	api.Unimplemented

	trips        *trip.Service
	pinger       Pinger
	queryTimeout time.Duration
}

func NewHandler(trips *trip.Service, pinger Pinger, queryTimeout time.Duration) *Handler {
	return &Handler{
		trips:        trips,
		pinger:       pinger,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	if !hasJSONContentType(r) {
		writeInvalidRequest(w, r)
		return
	}

	body, err := decodeCreateTripBody(w, r)
	if err != nil {
		writeInvalidRequest(w, r)
		return
	}

	result, err := h.trips.Create(r.Context(), trip.CreateInput{
		UserID:         body.UserId,
		DriverID:       body.DriverId,
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
		IdempotencyKey: params.IdempotencyKey,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	statusCode := http.StatusCreated
	if result.Replayed {
		statusCode = http.StatusOK
	} else {
		w.Header().Set("Location", "/api/v1/trips/"+result.Trip.ID.String())
	}
	writeJSONResponse(w, statusCode, toAPITrip(result.Trip))
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	result, err := h.trips.Get(r.Context(), tripID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, toAPITrip(result))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	result, err := h.trips.Finish(r.Context(), tripID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, toAPITrip(result))
}

func (h *Handler) HandleParameterError(w http.ResponseWriter, r *http.Request, _ error) {
	writeInvalidRequest(w, r)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if h.pinger == nil || h.pinger.Ping(ctx) != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}

	writeJSONResponse(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func decodeCreateTripBody(
	w http.ResponseWriter,
	r *http.Request,
) (api.CreateTripJSONRequestBody, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCreateTripBodyBytes))

	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return api.CreateTripJSONRequestBody{}, fmt.Errorf("decode request body: %w", err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return api.CreateTripJSONRequestBody{}, errors.New("request body must contain one JSON value")
		}

		return api.CreateTripJSONRequestBody{}, fmt.Errorf("decode trailing request data: %w", err)
	}

	fields, err := validateObjectFields(
		raw,
		"user_id",
		"driver_id",
		"start_point",
		"end_point",
		"price",
	)
	if err != nil {
		return api.CreateTripJSONRequestBody{}, err
	}

	if _, err := validateObjectFields(fields["start_point"], "latitude", "longitude"); err != nil {
		return api.CreateTripJSONRequestBody{}, fmt.Errorf("validate start_point: %w", err)
	}
	if _, err := validateObjectFields(fields["end_point"], "latitude", "longitude"); err != nil {
		return api.CreateTripJSONRequestBody{}, fmt.Errorf("validate end_point: %w", err)
	}

	strictDecoder := json.NewDecoder(bytes.NewReader(raw))
	strictDecoder.DisallowUnknownFields()

	var body api.CreateTripJSONRequestBody
	if err := strictDecoder.Decode(&body); err != nil {
		return api.CreateTripJSONRequestBody{}, fmt.Errorf("decode typed request body: %w", err)
	}

	return body, nil
}

func validateObjectFields(
	raw json.RawMessage,
	requiredFields ...string,
) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		if err != nil {
			return nil, fmt.Errorf("decode JSON object: %w", err)
		}

		return nil, errors.New("JSON value must be a non-null object")
	}

	allowedFields := make(map[string]struct{}, len(requiredFields))
	for _, field := range requiredFields {
		allowedFields[field] = struct{}{}
	}

	for field := range object {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("unknown field %q", field)
		}
	}

	for _, field := range requiredFields {
		value, ok := object[field]
		if !ok {
			return nil, fmt.Errorf("required field %q is missing", field)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("required field %q must not be null", field)
		}
	}

	return object, nil
}

func hasJSONContentType(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func toAPITrip(value trip.Trip) api.Trip {
	return api.Trip{
		Id:       value.ID,
		UserId:   value.UserID,
		DriverId: value.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  value.StartLatitude,
			Longitude: value.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  value.EndLatitude,
			Longitude: value.EndLongitude,
		},
		Price:          value.Price,
		Status:         api.TripStatus(value.Status),
		StartedAt:      value.StartedAt,
		FinishedAt:     value.FinishedAt,
		LastPositionAt: nil,
	}
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var joinedError interface {
		Unwrap() []error
	}
	if errors.As(err, &joinedError) {
		writeInternalError(w, r, err)
		return
	}

	switch {
	case errors.Is(err, trip.ErrInvalidTrip):
		writeInvalidRequest(w, r)
	case errors.Is(err, trip.ErrTripNotFound):
		writeProblem(
			w,
			r,
			http.StatusNotFound,
			"https://tripgo.example/problems/trip-not-found",
			"Trip not found",
			"Trip was not found",
			"trip_not_found",
		)
	case errors.Is(err, trip.ErrTripCompleted):
		writeProblem(
			w,
			r,
			http.StatusConflict,
			"https://tripgo.example/problems/trip-completed",
			"Trip completed",
			"Operation is not allowed for a completed trip",
			"trip_completed",
		)
	case errors.Is(err, trip.ErrDriverBusy):
		writeProblem(
			w,
			r,
			http.StatusConflict,
			"https://tripgo.example/problems/driver-busy",
			"Driver busy",
			"Driver already has an active trip",
			"driver_busy",
		)
	case errors.Is(err, trip.ErrIdempotencyConflict):
		writeProblem(
			w,
			r,
			http.StatusConflict,
			"https://tripgo.example/problems/idempotency-conflict",
			"Idempotency conflict",
			"Idempotency-Key was already used with a different request body",
			"idempotency_conflict",
		)
	default:
		writeInternalError(w, r, err)
	}
}

func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(
		r.Context(),
		"trip request failed",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	writeProblem(
		w,
		r,
		http.StatusInternalServerError,
		"https://tripgo.example/problems/internal-error",
		"Internal Server Error",
		"Internal server error",
		"internal_error",
	)
}

func writeInvalidRequest(w http.ResponseWriter, r *http.Request) {
	writeProblem(
		w,
		r,
		http.StatusBadRequest,
		"https://tripgo.example/problems/invalid-request",
		"Invalid request",
		"Request validation failed",
		"invalid_request",
	)
}

func writeProblem(
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	problemType string,
	title string,
	detail string,
	code string,
) {
	instance := r.URL.Path
	writeResponse(w, statusCode, "application/problem+json", api.Problem{
		Type:     problemType,
		Title:    title,
		Status:   int32(statusCode),
		Detail:   &detail,
		Instance: &instance,
		Code:     code,
	})
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, value any) {
	writeResponse(w, statusCode, "application/json", value)
}

func writeResponse(w http.ResponseWriter, statusCode int, contentType string, value any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode HTTP response", "error", err)
	}
}
