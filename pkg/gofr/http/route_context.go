package http

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

// routeTemplateCtxKey is the private context key under which the trie router
// stores the matched route template (e.g. "/users/{id}"). It exists because,
// once the trie router bypasses mux's ServeHTTP, mux.CurrentRoute(r) is no
// longer populated, so the template must be carried some other way.
type routeTemplateCtxKey struct{}

// withRouteTemplate returns r carrying tmpl as the matched route template. An
// empty template is a no-op so unmatched requests do not pay for a context copy.
func withRouteTemplate(r *http.Request, tmpl string) *http.Request {
	if tmpl == "" {
		return r
	}

	return r.WithContext(context.WithValue(r.Context(), routeTemplateCtxKey{}, tmpl))
}

// RouteTemplate returns the matched route template for r (e.g. "/users/{id}"),
// or "" if none is available. It is the router-agnostic replacement for
// mux.CurrentRoute(r).GetPathTemplate() and works under both routers: the trie
// router records the template in the request context (it bypasses mux's
// ServeHTTP, so mux.CurrentRoute is nil), while the default mux path exposes it
// via mux.CurrentRoute. Callers get consistent behavior without knowing which
// router is active.
func RouteTemplate(r *http.Request) string {
	if tmpl, _ := r.Context().Value(routeTemplateCtxKey{}).(string); tmpl != "" {
		return tmpl
	}

	if route := mux.CurrentRoute(r); route != nil {
		if tmpl, err := route.GetPathTemplate(); err == nil {
			return tmpl
		}
	}

	return ""
}
