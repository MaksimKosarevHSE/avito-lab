package middleware

import (
	"net/http"

	"github.com/MaksimKosarevHSE/avito-lab/internal/http/domainerror"
)

func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		panic("HTTP request body limit must be positive")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				domainerror.WriteInvalidRequest(w, r, "request body is too large")
				return
			}

			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
