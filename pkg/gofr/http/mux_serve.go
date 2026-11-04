package http

import (
	"context"
	"net/http"
	"path"
	"sync"
	"sync/atomic"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/internal/quietspan"
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
// matched into, read back for mux's two keys. When fastMatch decided the request, fast is the
// route it chose and path the path it matched; the vars map is then built on first read.
type muxState struct {
	context.Context
	match mux.RouteMatch

	fast  *fastRoute
	path  string
	once  sync.Once
	built atomic.Bool

	// qs holds the request span when Tracer, the handler serveMux calls, claims it (see
	// quietspan.Host).
	qs quietspan.Host
}

// armFor arms the span slot when h is Tracer: only the handler serveMux calls, and nothing outside
// it, holds the request carrying s, so the span may be filled in place.
func (s *muxState) armFor(h http.Handler) {
	if _, ok := h.(quietspan.Claimant); ok {
		s.qs.Arm()
	}
}

// QuietHost implements quietspan.Hoster.
func (s *muxState) QuietHost() *quietspan.Host { return &s.qs }

// Value answers mux's keys as the two context values mux stores would: the vars map and the
// route, typed, even when nil.
func (s *muxState) Value(key any) any {
	switch key {
	case muxVarsKey:
		if s.fast != nil {
			s.once.Do(s.buildVars)
		}

		return s.match.Vars
	case muxRouteKey:
		return s.match.Route
	}

	if v, ok := s.qs.Lookup(key); ok {
		return v
	}

	return s.Context.Value(key)
}

// buildVars builds the map mux would have: one entry per variable, a later name overwriting an
// earlier one, and an empty map, not nil, for a route without variables.
func (s *muxState) buildVars() {
	vars := make(map[string]string, len(s.fast.names))
	s.fast.match(s.path, vars)
	s.match.Vars = vars
	s.built.Store(true)
}

// pathParam answers one variable of the route fastMatch decided, without building the vars map.
// It can only while nobody has read the map, and so possibly changed it: ok is false otherwise,
// and for a request mux decided, and the caller must read the map.
func (s *muxState) pathParam(key string) (string, bool) {
	if s.fast == nil || s.built.Load() {
		return "", false
	}

	return s.fast.value(s.path, key), true
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

	if rou.fastMatch(r, s) || rou.Router.Match(r, &s.match) {
		handler = s.match.Handler
		r = r.WithContext(s)

		s.armFor(handler)
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
