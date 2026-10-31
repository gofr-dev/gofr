// Package stablemw marks the handlers GoFr's framework middlewares return, so that the http router
// can tell them apart from a user's. It is internal so the marker does not become public API.
package stablemw

import "net/http"

// Handler is the http.HandlerFunc GoFr's framework middlewares return. Its type tells the router
// that the constructor which built it depends only on the inner handler, so the mux matcher can
// build that layer once per route instead of on every request. Every middleware returning one must
// keep that property: anything it reads per request belongs inside the handler, not in the
// constructor.
type Handler func(http.ResponseWriter, *http.Request)

func (f Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { f(w, r) }

// Is reports whether h is a Handler.
func Is(h http.Handler) bool {
	_, ok := h.(Handler)

	return ok
}
