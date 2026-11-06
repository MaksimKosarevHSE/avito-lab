package get

import (
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	common_http_util "github.com/MaksimKosarevHSE/avito-lab/internal/http/common"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

type Handler struct {
	service tripGetter
}

func New(service tripGetter) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	result, err := h.service.Get(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	common_http_util.WriteJSONResponse(w, r, http.StatusOK, toAPITrip(result))
}
