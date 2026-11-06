package create

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	common_http_util "github.com/MaksimKosarevHSE/avito-lab/internal/http/common"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

type Handler struct {
	service tripCreator
}

func New(service tripCreator) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	body, err := decodeCreateTripBody(r)
	if err != nil {
		domainerror.WriteInvalidRequest(w, r, "request body contains invalid JSON")
		return
	}

	result, err := h.service.Create(r.Context(), toCreateInput(body, params))
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	statusCode := http.StatusCreated
	if result.Replayed {
		statusCode = http.StatusOK
	} else {
		w.Header().Set("Location", "/api/v1/trips/"+result.Trip.ID.String())
	}

	common_http_util.WriteJSONResponse(w, r, statusCode, toAPITrip(result.Trip))
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
