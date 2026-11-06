package finish

import (
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	common_http_util "github.com/MaksimKosarevHSE/avito-lab/internal/http/common"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

type Handler struct {
	service tripFinisher
}

func New(service tripFinisher) *Handler {
	return &Handler{service: service}
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	result, err := h.service.Finish(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	common_http_util.WriteJSONResponse(w, r, http.StatusOK, toAPITrip(result))
}
