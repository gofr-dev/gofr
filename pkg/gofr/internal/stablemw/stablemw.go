// Package stablemw marks the middlewares GoFr registers itself, so that the http router can tell
// them apart from an application's. It is internal so the mark does not become public API.
package stablemw

import (
	"net/http"
	"reflect"
)

// Mark returns mw, marked as one of GoFr's own middlewares: a constructor that depends only on the
// inner handler it is given, so the mux matcher may build its layer once per route instead of on
// every request. It must only wrap such constructors; anything they read per request belongs
// inside the handler they return.
//
// The mark is the identity of the closure below. Only this package can create it, so no
// application middleware can carry it -- whatever its constructor returns. Mark is never inlined
// because an inlined copy would compile that closure again under another identity.
//
//go:noinline
func Mark(mw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler { return mw(inner) }
}

// Is reports whether mw was returned by Mark. It is called when a route's chain is built, not per
// request.
func Is(mw func(http.Handler) http.Handler) bool {
	return mw != nil && reflect.ValueOf(mw).Pointer() == reflect.ValueOf(Mark(nil)).Pointer()
}
