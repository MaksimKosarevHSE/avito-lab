package common

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func WriteJSONResponse(w http.ResponseWriter, r *http.Request, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "encode HTTP response", "error", err)
	}
}
