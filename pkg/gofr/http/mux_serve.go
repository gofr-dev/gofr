package http

import (
	"context"
	"net/http"
	"path"

	"github.com/gorilla/mux"
)

// mux.Router.ServeHTTP stores the matched route's vars and the route itself in two context
// values, each with its own shallow copy of the request, after allocating the RouteMatch it
// matched into: five allocations per request on top of the match itself. serveMux does what
// that method does, in the same order, with one allocation that holds the RouteMatch and
// answers both of mux's context keys, and one copy of the request.
//
// Matching, the 404/405 choice and the middleware mux applies are still mux's own: serveMux
// calls the promoted Match, so a custom NotFoundHandler or MethodNotAllowedHandler, a
// subrouter, a MatcherFunc or a middleware registered on the embedded router behave as
// before. mux.Vars and mux.CurrentRoute return the same values they did.

// muxKeys are the keys mux.Vars and mux.CurrentRoute read. mux does not export them, so they
// are learned once by asking each function about a context that records the key it is asked
// for. If that ever fails to find two distinct keys, serveMux hands every request to mux.
//
//nolint:gochecknoglobals // set once at init from mux's own lookups; read-only afterwards.
var muxVarsKey, muxRouteKey = probeMuxKeys()

// keyProbe is a context that records the last key it is asked for and holds no values.
type keyProbe struct {
	context.Context
	key any
}

func (p *keyProbe) Value(key any) any {
	p.key = key

	return nil
}

func probeMuxKeys() (vars, route any) {
	p := &keyProbe{Context: context.Background()}
	r := (&http.Request{}).WithContext(p)

	_ = mux.Vars(r)
	vars = p.key

	p.key = nil
	_ = mux.CurrentRoute(r)
	route = p.key

	if vars == nil || route == nil || vars == route {
		return nil, nil
	}

	return vars, route
}

// Prove the probed keys round-trip through muxState before serveMux relies on them.
func init() { //nolint:gochecknoinits // validates muxVarsKey and muxRouteKey once, before any request.
	s := &muxState{Context: context.Background()}
	s.match.Vars = map[string]string{"k": "v"}
	s.match.Route = new(mux.Route)
	r := (&http.Request{}).WithContext(s)

	if mux.CurrentRoute(r) != s.match.Route || mux.Vars(r)["k"] != "v" {
		muxVarsKey, muxRouteKey = nil, nil
	}
}

// muxState is the request context serveMux installs: its parent plus the RouteMatch mux
// matched into, read back for mux's two keys.
type muxState struct {
	context.Context
	match mux.RouteMatch
}

// Value answers mux's keys as the two context values mux stores would: the vars map and the
// route, typed, even when nil.
func (s *muxState) Value(key any) any {
	switch key {
	case muxVarsKey:
		return s.match.Vars
	case muxRouteKey:
		return s.match.Route
	}

	return s.Context.Value(key)
}

// serveMux is mux.Router.ServeHTTP with the match and its context state in one allocation.
func (rou *Router) serveMux(w http.ResponseWriter, r *http.Request) {
	// mux redirects a path that is not clean, unless SkipClean is set; that state is not
	// readable here, so such a path is left to mux. normalizePath has already cleaned
	// r.URL.Path, so this only reaches mux for an escaped path that is not clean.
	if muxVarsKey == nil || !muxClean(r.URL.Path) || (r.URL.RawPath != "" && !muxClean(r.URL.EscapedPath())) {
		rou.Router.ServeHTTP(w, r)

		return
	}

	s := &muxState{Context: r.Context()}

	var handler http.Handler

	if rou.Router.Match(r, &s.match) {
		handler = s.match.Handler
		r = r.WithContext(s)
	}

	//nolint:errorlint,err113 // mux compares with ==; errors.Is would answer a wrapped error differently.
	if handler == nil && s.match.MatchErr == mux.ErrMethodMismatch {
		handler = http.HandlerFunc(muxMethodNotAllowed)
	}

	if handler == nil {
		handler = http.NotFoundHandler()
	}

	handler.ServeHTTP(w, r)
}

// muxMethodNotAllowed is what mux answers a method mismatch with when the router has no
// MethodNotAllowedHandler.
func muxMethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// muxClean reports whether mux's cleanPath leaves p unchanged, that is whether mux would
// serve p rather than redirect it.
func muxClean(p string) bool {
	if p == "" || p[0] != '/' {
		return false
	}

	np := path.Clean(p)
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}

	return np == p
}
