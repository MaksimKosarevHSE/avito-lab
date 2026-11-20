package get

import (
	"log/slog"
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/api"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/common"
	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

type Handler struct {
	positionGetter positionGetter

	response *common.ResponseWriter
	problems *domainerror.Writer
}

func New(
	positionGetter positionGetter,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		positionGetter: positionGetter,
		response:       common.NewResponseWriter(logger),
		problems:       domainerror.NewWriter(logger),
	}
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	positions, err := h.positionGetter.GetList(r.Context(), tripID)
	if err != nil {
		h.problems.Write(w, r, err)
		return
	}
	h.response.WriteJSON(w, r, http.StatusOK, toAPIPositionList(positions))
}
