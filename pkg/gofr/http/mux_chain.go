package http

import (
	"net/http"
	"sync/atomic"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/internal/stablemw"
)

// The default (mux) matcher composes the middleware chain inside mux.Router.Match, calling every
// middleware constructor on every request. GoFr's own five each returned a fresh closure there, five
// allocations per request that rebuild the same chain every time.
//
// The fix keeps mux in charge of serving, so mux.Vars, mux.CurrentRoute and the 404/405 handlers
// behave as before, and the promoted Match still returns the matched handler wrapped in the whole
// chain (a user constructor now runs when that handler is served rather than inside Match). What
// changes is what mux composes: Use hands mux a single middleware, dispatch, in place of the list. For a route GoFr registered itself
// (see ownedHandler), dispatch returns a chain built once and kept on the route; for any other
// route it composes the list per request, exactly as mux did.
//
// Only the constructors GoFr registered itself through stablemw.Mark are built once. An
// application's constructor still runs on every request, as it always has under mux: it may keep
// per-request state in its closure, or return a different handler from one request to the next
// (for example next itself while a flag is off), and memoising it would change what it does. The
// mark is on the registered constructor, not on the handler it returns, so a constructor that
// passes next through or calls a framework constructor itself is never mistaken for GoFr's.
// See buildMuxChain.

// ownedHandler is the handler Add installs on a route under the mux matcher. It serves the
// handler it wraps, and it is a pointer of a type only this package creates, which is what lets
// dispatch recognize the route's own handler in a single type assertion: the handler GoFr's
// routes carry is a struct with func fields, which cannot be compared or used as a map key.
type ownedHandler struct {
	h     http.Handler
	chain atomic.Pointer[muxChain]
}

func (o *ownedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { o.h.ServeHTTP(w, r) }

// muxChain is a route's composed chain and the middleware generation it was built from.
type muxChain struct {
	gen uint64
	h   http.Handler
}

// dispatch is the only middleware mux sees in mux mode. mux calls it from Match with the matched
// handler, on every request, after the route matched without error -- the same point it used to
// apply the whole list.
func (rou *Router) dispatch(h http.Handler) http.Handler {
	o, ok := h.(*ownedHandler)
	if !ok {
		return composeMiddleware(rou.mws, h)
	}

	gen := rou.gen.Load()
	if c := o.chain.Load(); c != nil && c.gen == gen {
		return c.h
	}

	served, cached := buildMuxChain(rou.mws, o)

	// Concurrent first requests may each build; the chains are equivalent and the last store wins.
	o.chain.Store(&muxChain{gen: gen, h: cached})

	return served
}

// buildMuxChain composes mws over inner for the request being served and returns that chain
// (served) together with the one to keep on the route (cached).
//
// The chain is composed inner to outer as mux would, one constructor call per middleware, and
// that composition serves this request, so no user constructor is called more than it was. A
// layer is pure when its constructor carries stablemw's mark. The pure layers at the inner end
// are kept as they are; the pure layers at the outer end are composed again over a perRequest
// layer that rebuilds everything between them on every request. In a default GoFr app every
// layer is pure and served is cached.
func buildMuxChain(mws []mux.MiddlewareFunc, inner http.Handler) (served, cached http.Handler) {
	layers := make([]http.Handler, len(mws))

	h := inner
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
		layers[i] = h
	}

	// lo..hi is the impure run: [0, lo) and (hi, len) are pure.
	lo, hi := 0, len(mws)-1
	for lo < len(mws) && stablemw.Is(mws[lo]) {
		lo++
	}

	if lo == len(mws) {
		return h, h
	}

	for hi >= lo && stablemw.Is(mws[hi]) {
		hi--
	}

	tail := inner
	if hi+1 < len(mws) {
		tail = layers[hi+1]
	}

	middle := &perRequest{mws: mws[lo : hi+1 : hi+1], inner: tail}

	return h, composeMiddleware(mws[:lo:lo], middle)
}

// perRequest composes its middlewares over inner on every request, as mux did for the whole chain.
type perRequest struct {
	mws   []mux.MiddlewareFunc
	inner http.Handler
}

func (p *perRequest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	composeMiddleware(p.mws, p.inner).ServeHTTP(w, r)
}
