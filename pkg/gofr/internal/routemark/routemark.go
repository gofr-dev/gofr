// Package routemark records which route a request matched and which route is GoFr's own 404
// catch-all, so that the http router, the RBAC middleware and the RBAC route check all ask the
// same question the same way. It is internal so none of this becomes public API.
package routemark

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

// catchAllName is the name the catch-all route is registered under. mux keeps it in a plain field of
// the route, so asking for it costs a string compare and no allocation, and no registry is needed
// that would have to outlive or be shared between routers.
const catchAllName = "gofr.catch-all"

// ctxKey is the private context key under which the trie router stores the matched route. It
// exists because, once the trie router bypasses mux's ServeHTTP, mux.CurrentRoute(r) is no longer
// populated. A pointer is stored rather than the template string: it fits in an interface without
// an allocation.
type ctxKey struct{}

// MarkCatchAll marks route as GoFr's catch-all, the route that answers 404 for every request no
// other route matched, and returns it. Only that route is marked: a route an application registers
// at "/", or a static directory served from "/", has the same template and is a real route.
func MarkCatchAll(route *mux.Route) *mux.Route {
	return route.Name(catchAllName)
}

// IsCatchAll reports whether route is the route MarkCatchAll marked. A nil route is not.
func IsCatchAll(route *mux.Route) bool {
	return route != nil && route.GetName() == catchAllName
}

// With returns r carrying route as the matched route. A nil route is a no-op so unmatched requests
// do not pay for a context copy.
func With(r *http.Request, route *mux.Route) *http.Request {
	if route == nil {
		return r
	}

	return r.WithContext(context.WithValue(r.Context(), ctxKey{}, route))
}

// Matched returns the route that matched r under either router, or nil.
func Matched(r *http.Request) *mux.Route {
	if route, _ := r.Context().Value(ctxKey{}).(*mux.Route); route != nil {
		return route
	}

	return mux.CurrentRoute(r)
}

// CatchAllAnswered reports whether GoFr's catch-all is the route that matched r.
func CatchAllAnswered(r *http.Request) bool {
	return IsCatchAll(Matched(r))
}
