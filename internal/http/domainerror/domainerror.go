package domainerror

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	domaintrip "github.com/MaksimKosarevHSE/avito-lab/internal/domain/trip"
)

const (
	problemBaseURL       = "https://tripgo.example/problems/"
	invalidRequestDetail = "Request does not match the API contract"
)

type Writer struct {
	logger *slog.Logger
}

func NewWriter(logger *slog.Logger) *Writer {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &Writer{logger: logger}
}

func Problem(err error) (int, api.Problem) {
	var joinedError interface {
		Unwrap() []error
	}
	if errors.As(err, &joinedError) {
		return internalError()
	}

	switch {
	case errors.Is(err, domaintrip.ErrInvalidTrip):
		return newProblem(
			http.StatusBadRequest,
			"invalid-request",
			"Invalid request",
			err.Error(),
			"invalid_request",
		)
	case errors.Is(err, domaintrip.ErrTripNotFound):
		return newProblem(
			http.StatusNotFound,
			"trip-not-found",
			"Trip not found",
			"Trip was not found",
			"trip_not_found",
		)
	case errors.Is(err, domaintrip.ErrTripCompleted):
		return newProblem(
			http.StatusConflict,
			"trip-completed",
			"Trip completed",
			"Operation is not allowed for a completed trip",
			"trip_completed",
		)
	case errors.Is(err, domaintrip.ErrDriverBusy):
		return newProblem(
			http.StatusConflict,
			"driver-busy",
			"Driver busy",
			"Driver already has an active trip",
			"driver_busy",
		)
	case errors.Is(err, domaintrip.ErrIdempotencyConflict):
		return newProblem(
			http.StatusConflict,
			"idempotency-conflict",
			"Idempotency conflict",
			"Idempotency-Key was already used with a different request body",
			"idempotency_conflict",
		)
	default:
		return internalError()
	}
}

func InvalidRequest(detail string) (int, api.Problem) {
	return newProblem(
		http.StatusBadRequest,
		"invalid-request",
		"Invalid request",
		detail,
		"invalid_request",
	)
}

func (writer *Writer) Write(w http.ResponseWriter, r *http.Request, err error) {
	statusCode, problem := Problem(err)
	if statusCode == http.StatusInternalServerError {
		writer.logger.ErrorContext(
			r.Context(),
			"HTTP request failed",
			"method", r.Method,
			"error", err,
		)
	}

	writer.write(w, r, statusCode, problem)
}

func (writer *Writer) WriteInvalidRequest(
	w http.ResponseWriter,
	r *http.Request,
	detail string,
) {
	statusCode, problem := InvalidRequest(detail)
	writer.write(w, r, statusCode, problem)
}

func (writer *Writer) WriteInternal(w http.ResponseWriter, r *http.Request) {
	statusCode, problem := internalError()
	writer.write(w, r, statusCode, problem)
}

func (writer *Writer) write(
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	problem api.Problem,
) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		writer.logger.ErrorContext(r.Context(), "encode HTTP problem response", "error", err)
	}
}

func internalError() (int, api.Problem) {
	return newProblem(
		http.StatusInternalServerError,
		"internal-error",
		"Internal Server Error",
		"Internal server error",
		"internal_error",
	)
}

func newProblem(
	statusCode int32,
	problemType string,
	title string,
	detail string,
	code string,
) (int, api.Problem) {
	return int(statusCode), api.Problem{
		Type:   problemBaseURL + problemType,
		Title:  title,
		Status: statusCode,
		Detail: &detail,
		Code:   code,
	}
}
