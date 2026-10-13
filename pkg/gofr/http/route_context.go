package http

import (
	"context"
	"net/http"
	"regexp/syntax"

	"github.com/gorilla/mux"
)

// routeCtxKey is the private context key under which the trie router stores
// the matched route. It exists because, once the trie router bypasses mux's
// ServeHTTP, mux.CurrentRoute(r) is no longer populated, so the route must be
// carried some other way. A pointer is stored rather than the template string:
// it fits in an interface without an allocation.
type routeCtxKey struct{}

// withRoute returns r carrying route as the matched route. A nil route is a
// no-op so unmatched requests do not pay for a context copy.
func withRoute(r *http.Request, route *mux.Route) *http.Request {
	if route == nil {
		return r
	}

	return r.WithContext(context.WithValue(r.Context(), routeCtxKey{}, route))
}

// matchedRoute returns the route that matched r under either router, or nil.
func matchedRoute(r *http.Request) *mux.Route {
	if route, _ := r.Context().Value(routeCtxKey{}).(*mux.Route); route != nil {
		return route
	}

	return mux.CurrentRoute(r)
}

// RouteTemplate returns the matched route template for r (e.g. "/users/{id}"),
// or "" if none is available. It is the router-agnostic replacement for
// mux.CurrentRoute(r).GetPathTemplate() and works under both routers: the trie
// router records the template in the request context (it bypasses mux's
// ServeHTTP, so mux.CurrentRoute is nil), while the default mux path exposes it
// via mux.CurrentRoute. Callers get consistent behavior without knowing which
// router is active.
func RouteTemplate(r *http.Request) string {
	if route := matchedRoute(r); route != nil {
		if tmpl, err := route.GetPathTemplate(); err == nil {
			return tmpl
		}
	}

	return ""
}

// IsPrefixRoute reports whether the route that matched r was registered with
// PathPrefix, so it serves every path below its template rather than the
// template alone. GoFr's catch-all is such a route at "/", which is how a caller
// tells it apart from an application route registered at "/": both have the
// template "/". It reports false when no route matched.
func IsPrefixRoute(r *http.Request) bool {
	route := matchedRoute(r)
	if route == nil {
		return false
	}

	rx, err := route.GetPathRegexp()
	if err != nil {
		return false
	}

	re, err := syntax.Parse(rx, syntax.Perl)
	if err != nil {
		return false
	}

	return !isAnchoredBothEnds(re)
}
