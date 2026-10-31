package middleware

import "net/http"

// stableHandler is the http.HandlerFunc GoFr's framework middlewares return. Its extra method
// tells the router that the constructor which built it depends only on the inner handler, so the
// mux matcher can build that layer once per route instead of on every request. Every middleware
// returning one must keep that property: anything it reads per request belongs inside the
// handler, not in the constructor.
type stableHandler func(http.ResponseWriter, *http.Request)

func (f stableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { f(w, r) }

// GoFrStableMiddleware marks the handler for the router; it does nothing.
func (stableHandler) GoFrStableMiddleware() {}
