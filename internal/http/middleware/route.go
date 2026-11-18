package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const unmatchedRoute = "unmatched"

func routePattern(r *http.Request) string {
	if pattern := currentRoutePattern(r); pattern != "" {
		return pattern
	}

	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		return matchRoute(routeContext.Routes, r)
	}

	return unmatchedRoute
}

func matchRoute(routes chi.Routes, r *http.Request) string {
	if pattern := currentRoutePattern(r); pattern != "" {
		return pattern
	}
	if routes == nil {
		return unmatchedRoute
	}

	requestPath := r.URL.Path
	if r.URL.RawPath != "" {
		requestPath = r.URL.RawPath
	}

	matchContext := chi.NewRouteContext()
	if routes.Match(matchContext, r.Method, requestPath) {
		if pattern := matchContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}

	return unmatchedRoute
}

func currentRoutePattern(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}

	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		return routeContext.RoutePattern()
	}

	return ""
}
