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
	positionCreator positionCreator
	responses       *common.ResponseWriter
	problems        *domainerror.Writer
}

func New(
	positionCreator positionCreator,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		positionCreator: positionCreator,
		responses:       common.NewResponseWriter(logger),
		problems:        domainerror.NewWriter(logger),
	}
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	body, err := decodeCreatePositionBody(r)
	if err != nil {
		h.problems.WriteInvalidRequest(w, r, "request body contains invalid JSON")
		return
	}
	created, err := h.positionCreator.Create(r.Context(), toCreateInput(body, tripID))
	if err != nil {
		h.problems.Write(w, r, err)
		return
	}
	h.responses.WriteJSON(w, r, http.StatusCreated, toAPIPosition(created))
}

func decodeCreatePositionBody(
	r *http.Request,
) (api.CreateTripPositionJSONRequestBody, error) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}

	var body api.CreateTripPositionJSONRequestBody
	if err := json.Unmarshal(payload, &body); err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}

	return body, nil
}
