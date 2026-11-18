package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/otel/trace"
)

type internalProblem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		panic("recover middleware requires a logger")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, state := withRequestState(r)
			status := 0
			committed := false
			trackedWriter := httpsnoop.Wrap(w, httpsnoop.Hooks{
				WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
					return func(code int) {
						if !committed {
							committed = true
							status = code
						}
						next(code)
					}
				},
				Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
					return func(body []byte) (int, error) {
						if !committed {
							committed = true
							status = http.StatusOK
						}
						return next(body)
					}
				},
				Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
					return func() {
						if !committed {
							committed = true
							status = http.StatusOK
						}
						next()
					}
				},
			})

			defer func() {
				panicValue := recover()
				if panicValue == nil {
					return
				}

				logContext := r.Context()
				if spanContext := state.getSpanContext(); spanContext.IsValid() {
					logContext = trace.ContextWithSpanContext(logContext, spanContext)
				}
				logger.ErrorContext(
					logContext,
					"panic recovered",
					"panic", panicValue,
					"stack", string(debug.Stack()),
				)

				if !committed {
					if err := writeInternalProblem(trackedWriter); err != nil {
						logger.ErrorContext(
							logContext,
							"encode HTTP problem response",
							"error", err,
						)
					}
				}
				if status == 0 {
					status = http.StatusInternalServerError
				}
				state.runPanicFinalizers(status)
			}()

			next.ServeHTTP(trackedWriter, r)
		})
	}
}

func writeInternalProblem(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusInternalServerError)
	return json.NewEncoder(w).Encode(internalProblem{
		Type:   "https://tripgo.example/problems/internal-error",
		Title:  "Internal Server Error",
		Status: http.StatusInternalServerError,
		Detail: "Internal server error",
		Code:   "internal_error",
	})
}
