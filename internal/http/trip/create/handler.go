package create

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/common"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

type Handler struct {
	service   tripCreator
	responses *common.ResponseWriter
	problems  *domainerror.Writer
}

func New(service tripCreator, logger *slog.Logger) *Handler {
	return &Handler{
		service:   service,
		responses: common.NewResponseWriter(logger),
		problems:  domainerror.NewWriter(logger),
	}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	body, err := decodeCreateTripBody(r)
	if err != nil {
		h.problems.WriteInvalidRequest(w, r, "request body contains invalid JSON")
		return
	}

	result, err := h.service.Create(r.Context(), toCreateInput(body, params))
	if err != nil {
		h.problems.Write(w, r, err)
		return
	}

	statusCode := http.StatusCreated
	if result.Replayed {
		statusCode = http.StatusOK
	} else {
		w.Header().Set("Location", "/api/v1/trips/"+result.Trip.ID.String())
	}

	h.responses.WriteJSON(w, r, statusCode, toAPITrip(result.Trip))
}

func decodeCreateTripBody(
	r *http.Request,
) (api.CreateTripJSONRequestBody, error) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return api.CreateTripJSONRequestBody{}, err
	}

	var body api.CreateTripJSONRequestBody
	if err := json.Unmarshal(payload, &body); err != nil {
		return api.CreateTripJSONRequestBody{}, err
	}

	return body, nil
}
