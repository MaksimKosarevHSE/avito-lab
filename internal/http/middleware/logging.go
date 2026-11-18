package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/felixge/httpsnoop"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		panic("logging middleware requires a logger")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			status := http.StatusOK
			writeLog := func(finalStatus int, panicked bool) {
				args := []any{
					"method", r.Method,
					"route", routePattern(r),
					"status", finalStatus,
					"duration_ms", time.Since(startedAt).Milliseconds(),
				}
				if panicked || finalStatus >= http.StatusInternalServerError {
					logger.ErrorContext(r.Context(), "request handled", args...)
				} else {
					logger.InfoContext(r.Context(), "request handled", args...)
				}
			}

			defer func() {
				panicValue := recover()
				if panicValue != nil {
					finalizer := func(finalStatus int) {
						writeLog(finalStatus, true)
					}
					if !requestStateFromContext(r.Context()).addPanicFinalizer(finalizer) {
						finalizer(http.StatusInternalServerError)
					}
					panic(panicValue)
				}

				writeLog(status, false)
			}()

			observed := httpsnoop.CaptureMetrics(next, w, r)
			status = observed.Code
		})
	}
}
