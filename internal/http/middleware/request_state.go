package middleware

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

type requestStateKey struct{}

type requestState struct {
	spanContext     trace.SpanContext
	panicFinalizers []func(status int)
}

func withRequestState(r *http.Request) (*http.Request, *requestState) {
	state := &requestState{}
	ctx := context.WithValue(r.Context(), requestStateKey{}, state)

	return r.WithContext(ctx), state
}

func requestStateFromContext(ctx context.Context) *requestState {
	state, _ := ctx.Value(requestStateKey{}).(*requestState)
	return state
}

func (s *requestState) setSpanContext(spanContext trace.SpanContext) {
	if s == nil {
		return
	}

	s.spanContext = spanContext
}

func (s *requestState) getSpanContext() trace.SpanContext {
	if s == nil {
		return trace.SpanContext{}
	}

	return s.spanContext
}

func (s *requestState) addPanicFinalizer(finalizer func(status int)) bool {
	if s == nil || finalizer == nil {
		return false
	}

	s.panicFinalizers = append(s.panicFinalizers, finalizer)

	return true
}

func (s *requestState) runPanicFinalizers(status int) {
	if s == nil {
		return
	}

	finalizers := s.panicFinalizers
	s.panicFinalizers = nil

	for _, finalizer := range finalizers {
		finalizer(status)
	}
}
