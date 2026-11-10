package middleware

import (
	"context"
	"net/http"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

type requestStateKey struct{}

type requestState struct {
	mu              sync.RWMutex
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

	s.mu.Lock()
	s.spanContext = spanContext
	s.mu.Unlock()
}

func (s *requestState) getSpanContext() trace.SpanContext {
	if s == nil {
		return trace.SpanContext{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.spanContext
}

func (s *requestState) addPanicFinalizer(finalizer func(status int)) bool {
	if s == nil || finalizer == nil {
		return false
	}

	s.mu.Lock()
	s.panicFinalizers = append(s.panicFinalizers, finalizer)
	s.mu.Unlock()

	return true
}

func (s *requestState) runPanicFinalizers(status int) {
	if s == nil {
		return
	}

	s.mu.Lock()
	finalizers := append([]func(int){}, s.panicFinalizers...)
	s.panicFinalizers = nil
	s.mu.Unlock()

	for _, finalizer := range finalizers {
		finalizer(status)
	}
}
