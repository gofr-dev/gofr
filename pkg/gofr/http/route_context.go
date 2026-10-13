package http

import (
	"net/http"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/internal/routemark"
)

// withRoute returns r carrying route as the matched route. It exists because, once the trie
// router bypasses mux's ServeHTTP, mux.CurrentRoute(r) is no longer populated, so the route must be
// carried some other way. A nil route is a no-op so unmatched requests do not pay for a context copy.
func withRoute(r *http.Request, route *mux.Route) *http.Request {
	return routemark.With(r, route)
}

// RouteTemplate returns the matched route template for r (e.g. "/users/{id}"),
// or "" if none is available. It is the router-agnostic replacement for
// mux.CurrentRoute(r).GetPathTemplate() and works under both routers: the trie
// router records the template in the request context (it bypasses mux's
// ServeHTTP, so mux.CurrentRoute is nil), while the default mux path exposes it
// via mux.CurrentRoute. Callers get consistent behavior without knowing which
// router is active.
func RouteTemplate(r *http.Request) string {
	if route := routemark.Matched(r); route != nil {
		if tmpl, err := route.GetPathTemplate(); err == nil {
			return tmpl
		}
	}

	return ""
}
