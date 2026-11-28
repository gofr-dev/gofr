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

// builtInName is the name every route GoFr registers for itself carries (health, alive, favicon,
// OpenAPI/Swagger, the GraphQL playground). mux lets one name sit on many routes, and GoFr names no
// route otherwise, so the name is free to carry the mark.
const builtInName = "gofr.built-in"

// webSocketName is the name a route registered through App.WebSocket carries. The WebSocket
// upgrade middleware upgrades only a route carrying this mark: an Upgrade header on any other
// route must be ignored, or the hijacked connection is never closed and file descriptors leak
// (gofr#4635). A route is marked at registration time, never by path, so a plain route that
// happens to share a WebSocket route's template is not mistaken for one.
const webSocketName = "gofr.websocket"

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

// MarkWebSocket marks route as a WebSocket route, registered through App.WebSocket, and returns it.
// Only such a route may be upgraded to a WebSocket connection; see webSocketName.
func MarkWebSocket(route *mux.Route) *mux.Route {
	return route.Name(webSocketName)
}

// IsWebSocket reports whether route is one MarkWebSocket marked. A nil route is not.
func IsWebSocket(route *mux.Route) bool {
	return route != nil && route.GetName() == webSocketName
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

// MarkBuiltIn marks route as one GoFr registered for itself and returns it. GoFr tells these routes
// apart by where they were registered, not by path, so that an application route under
// /.well-known/ is not mistaken for one. Only unnamed routes created by App.add reach it.
func MarkBuiltIn(route *mux.Route) *mux.Route {
	return route.Name(builtInName)
}

// IsBuiltIn reports whether route is one MarkBuiltIn marked. A nil route is not.
func IsBuiltIn(route *mux.Route) bool {
	return route != nil && route.GetName() == builtInName
}
