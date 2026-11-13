package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const unmatchedRoute = "unmatched"

func RouteTemplate(routes chi.Routes, r *http.Request) string {
	requestPath := r.URL.Path
	if r.URL.RawPath != "" {
		requestPath = r.URL.RawPath
	}

	if routes != nil {
		matchContext := chi.NewRouteContext()
		if routes.Match(matchContext, r.Method, requestPath) {
			if pattern := matchContext.RoutePattern(); pattern != "" {
				return pattern
			}
		}
	}

	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}

		if routeContext.Routes != nil {
			matchContext := chi.NewRouteContext()
			if routeContext.Routes.Match(matchContext, r.Method, requestPath) {
				if pattern := matchContext.RoutePattern(); pattern != "" {
					return pattern
				}
			}
		}
	}

	if r.Pattern != "" {
		return r.Pattern
	}

	return unmatchedRoute
}

func optionalRoutes(routes []chi.Routes) chi.Routes {
	if len(routes) > 1 {
		panic("at most one Chi route table can be provided")
	}
	if len(routes) == 0 {
		return nil
	}

	return routes[0]
}

type routeCarrier interface {
	observabilityRoutes() chi.Routes
}

type handlerWithRoutes struct {
	http.Handler
	routes chi.Routes
}

func (h handlerWithRoutes) observabilityRoutes() chi.Routes {
	return h.routes
}

func routesForHandler(explicit chi.Routes, next http.Handler) chi.Routes {
	if explicit != nil {
		return explicit
	}
	if routes, ok := next.(chi.Routes); ok {
		return routes
	}
	if carrier, ok := next.(routeCarrier); ok {
		return carrier.observabilityRoutes()
	}

	return nil
}

func carryRoutes(handler http.Handler, routes chi.Routes) http.Handler {
	return handlerWithRoutes{Handler: handler, routes: routes}
}
