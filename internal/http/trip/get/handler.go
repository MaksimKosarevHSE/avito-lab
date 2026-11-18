package get

import (
	"log/slog"
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/common"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

type Handler struct {
	service   tripGetter
	responses *common.ResponseWriter
	problems  *domainerror.Writer
}

func New(service tripGetter, logger *slog.Logger) *Handler {
	return &Handler{
		service:   service,
		responses: common.NewResponseWriter(logger),
		problems:  domainerror.NewWriter(logger),
	}
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	result, err := h.service.Get(r.Context(), tripID)
	if err != nil {
		h.problems.Write(w, r, err)
		return
	}

	h.responses.WriteJSON(w, r, http.StatusOK, toAPITrip(result))
}
