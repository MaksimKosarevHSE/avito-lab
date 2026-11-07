package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
	createhttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/create"
	finishhttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/finish"
	gethttp "github.com/MaksimKosarevHSE/avito-lab/internal/http/trip/get"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	create       *createhttp.Handler
	get          *gethttp.Handler
	finish       *finishhttp.Handler
	pinger       pinger
	readyTimeout time.Duration
}

func NewHandler(
	createHandler *createhttp.Handler,
	getHandler *gethttp.Handler,
	finishHandler *finishhttp.Handler,
	pinger pinger,
	readyTimeout time.Duration,
) *Handler {
	return &Handler{
		create:       createHandler,
		get:          getHandler,
		finish:       finishHandler,
		pinger:       pinger,
		readyTimeout: readyTimeout,
	}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	h.create.CreateTrip(w, r, params)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.get.GetTrip(w, r, tripID)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.finish.FinishTrip(w, r, tripID)
}

func (h *Handler) HandleParameterError(w http.ResponseWriter, r *http.Request, err error) {
	domainerror.WriteValidationError(w, r, err)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, r, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.readyTimeout)
	defer cancel()

	if h.pinger == nil || h.pinger.Ping(ctx) != nil {
		writeJSONResponse(
			w,
			r,
			http.StatusServiceUnavailable,
			api.HealthResponse{Status: api.Unavailable},
		)
		return
	}

	writeJSONResponse(w, r, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func writeJSONResponse(
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	value any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.ErrorContext(r.Context(), "encode HTTP response", "error", err)
	}
}

var _ api.ServerInterface = (*Handler)(nil)
