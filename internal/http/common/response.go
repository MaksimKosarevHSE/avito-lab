package common

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ResponseWriter struct {
	logger *slog.Logger
}

func NewResponseWriter(logger *slog.Logger) *ResponseWriter {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &ResponseWriter{logger: logger}
}

func (rw *ResponseWriter) WriteJSON(
	w http.ResponseWriter,
	r *http.Request,
	statusCode int,
	body any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		rw.logger.ErrorContext(r.Context(), "encode HTTP response", "error", err)
	}
}
