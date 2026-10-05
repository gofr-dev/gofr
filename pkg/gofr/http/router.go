package http

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/logging"
)

const (
	DefaultSwaggerFileName       = "openapi.json"
	staticServerNotFoundFileName = "404.html"
	staticServerIndexFileName    = "index.html"

	// RouterEnvVar selects the route matcher. Unset (or any unrecognized value)
	// means MatcherMux, so the default behavior is unchanged.
	RouterEnvVar = "GOFR_ROUTER"

	// MatcherMux is gorilla/mux's linear scan — the default.
	MatcherMux = "mux"
	// MatcherTrie is the opt-in segment-trie index, O(path length) in the number
	// of registered routes.
	MatcherTrie = "trie"
)

// errReadPermissionDenied wraps fs.ErrPermission so that a file whose mode carries no read bit is
// reported the same way a real EACCES from os.Open is — the two reach respondWithFileError by
// different routes and must not answer differently.
var errReadPermissionDenied = fmt.Errorf("file does not have read permission: %w", fs.ErrPermission)

// Router is responsible for routing HTTP request.
type Router struct {
	mux.Router
	RegisteredRoutes *[]string

	// useTrie selects the O(path) trie matcher (GOFR_ROUTER=trie) over mux's
	// default O(n) linear scan. When false, ServeHTTP delegates to mux exactly
	// as before, so the default behavior is byte-for-byte unchanged.
	useTrie bool
	// idx is the trie index. It is built once, lazily, on the first request,
	// from the routes registered up to that point. This is correct for GoFr's
	// lifecycle: every route is registered during startup (app.GET/POST/...,
	// the GraphQL route, the static/catch-all handlers) before the server
	// accepts its first request, and GoFr does not add routes afterwards. A
	// route registered after the first request would not be reflected in the
	// trie index — a deliberate trade for a lock-free steady state, matching
	// GoFr's static-routing model. buildIdx guards that one-time build.
	//
	// AllowedMethods reads the same index in either mode, so with the mux matcher
	// it is built on the first request no route matched rather than the first
	// request. Matching itself stays mux's linear scan there.
	idx      *routeIndex
	buildIdx sync.Once
	// mws mirrors the middleware chain registered via Use, so the trie matcher
	// can apply it itself when it bypasses mux's ServeHTTP. In mux mode it is
	// unused (mux owns the chain) but kept in sync, costing nothing.
	//
	// Invariant: every middleware MUST be registered through (*Router).Use (or
	// UseMiddleware, which calls it). A direct call to the embedded
	// mux.Router.Use would bypass this slice and be silently dropped in trie
	// mode. All framework registration paths go through (*Router).Use, and the
	// MiddlewareParity differential test guards the resulting behavior.
	mws []mux.MiddlewareFunc

	// chains memoises the composed middleware chain per matched route.
	//
	// composeMiddleware calls every middleware CONSTRUCTOR, and each one returns a
	// fresh http.HandlerFunc closure. Doing that per request allocated one closure
	// per middleware on every request -- five of them in a default GoFr app, which
	// an allocation profile put at ~13% of all objects allocated while serving.
	// The composed chain depends only on the middleware slice and the matched
	// route's handler, and neither changes after startup, so it is built once per
	// route and reused.
	//
	// Keyed on *mux.Route rather than on the handler: mux sets match.Handler from
	// the route's own fixed handler field, so the route pointer identifies the
	// chain exactly, and unlike a handler it is always comparable. A handler
	// carrying func fields would panic as a map key.
	//
	// The same lifecycle assumption the trie index already makes applies here --
	// every route and every middleware is registered during startup, before the
	// first request. A middleware added afterwards would not appear in a chain
	// already cached for a route that had been served.
	chains sync.Map

	// own records the routes registered through Add, whose handler is the one Add
	// installed and never changes. Only those may use the chain cache.
	own sync.Map

	// cached reports whether any chain has been memoized yet, which is to say
	// whether the lifecycle assumption above has started to bite. It exists so
	// Use can say so rather than leaving a middleware silently not running.
	cached atomic.Bool

	// logger is optional and set by the server. Nothing on the request path uses
	// it; it exists so Use can report a late registration.
	logger logging.Logger
}

type Middleware func(handler http.Handler) http.Handler

// NewRouter creates a new Router instance.
func NewRouter() *Router {
	muxRouter := mux.NewRouter().StrictSlash(false).SkipClean(true)
	routes := make([]string, 0)
	r := &Router{
		Router:           *muxRouter,
		RegisteredRoutes: &routes,
		useTrie:          strings.EqualFold(os.Getenv(RouterEnvVar), MatcherTrie),
	}

	return r
}

// ServeHTTP implements [http.Handler] interface with path normalization.
func (rou *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	normalizePath(r)

	if rou.useTrie {
		rou.serveTrie(w, r)

		return
	}

	// Delegate to the underlying Gorilla Mux router.
	rou.Router.ServeHTTP(w, r)
}

// normalizePath canonicalizes r.URL.Path in place so routing sees a clean path
// (no "//", "/.", "/.." or non-root trailing slash), matching the behavior both
// the mux and trie matchers rely on.
func normalizePath(r *http.Request) {
	originalPath := r.URL.Path

	// Fast path: the vast majority of incoming paths are already canonical
	// ("/users/42", "/api/v1/things"). Skip the path.Clean + string ops in
	// that case so they only run for inputs that actually need normalizing.
	if isCleanPath(originalPath) {
		return
	}

	normalizedPath := path.Clean(originalPath)

	// path.Clean returns "." for empty paths, convert to "/" for HTTP routing
	if normalizedPath == "." {
		normalizedPath = "/"
	}

	// Ensure path starts with "/" for HTTP routing
	normalizedPath = "/" + strings.TrimLeft(normalizedPath, "/")

	// Only modify if path changed
	if originalPath != normalizedPath {
		r.URL.Path = normalizedPath
		if r.URL.RawPath != "" {
			r.URL.RawPath = normalizedPath
		}
	}
}

// serveTrie handles a request using the trie matcher: it matches (delegating the
// real decision to mux's Route.Match), restores the path params and route
// template that mux's own ServeHTTP would have set, then runs the matched
// handler through GoFr's middleware chain.
//
// The middleware chain wraps the matched handler ONLY. mux builds its chain
// inside Match, guarded by MatchErr == nil, so it never wraps the NotFound or
// MethodNotAllowed handlers — a 404/405 request is served by mux without the
// Tracer/Logging/CORS/Metrics chain running. serveTrie mirrors that exactly, so
// unmatched requests are not logged, measured, or CORS-answered (and the metrics
// path label is never populated from a raw, unbounded request path).
//
// Anything the trie does not match is handed to mux's own ServeHTTP rather than
// resolved here — see the comment on that call for why.
func (rou *Router) serveTrie(w http.ResponseWriter, r *http.Request) {
	var match mux.RouteMatch

	if rou.index().match(r, &match) && match.Handler != nil {
		rou.serveMatched(w, r, &match)

		return
	}

	// Unmatched by the trie: hand the request to mux's own ServeHTTP instead of
	// deciding 404-vs-405 here. This is the cold path — the response is already
	// an error — so mux's linear scan costs nothing that matters, and it buys two
	// things that reimplementing the decision cannot.
	//
	// Exactness. mux's 405 depends on state that routes which do NOT match the
	// request path still mutate: a later route whose method matcher succeeds
	// clears an ErrMethodMismatch left by an earlier one (gorilla/mux route.go),
	// and GoFr registers routes as Methods(m).Path(p), so the method matcher runs
	// first. Reproducing that outcome therefore requires visiting routes the trie
	// exists to skip. Delegating gets it exactly right instead of approximately.
	// The custom NotFoundHandler / MethodNotAllowedHandler and subrouter
	// semantics come along for free, and mux applies no middleware on this path,
	// so the parity the chain relies on is mux's own by construction.
	//
	// Safety. It also makes the index self-healing: if isIndexablePathRegexp ever
	// admitted a shape it should not have and the trie dropped a route that does
	// match, mux's full scan finds it here and serves it correctly. That turns the
	// one catastrophic failure mode of this design — a live route silently
	// 404ing — into a request that is merely slower, leaving the trie strictly an
	// accelerator.
	//
	// Inside a GoFr app this is unreachable: the PathPrefix("/") catch-all matches
	// every path and method, so the trie always has a candidate that matches.
	rou.Router.ServeHTTP(w, r)
}

// index returns the route index, building it on first use from the routes registered so far. See
// the idx field for why a one-time build is sound.
func (rou *Router) index() *routeIndex {
	rou.buildIdx.Do(func() {
		idx := newRouteIndex()
		idx.build(&rou.Router)
		rou.idx = idx
	})

	return rou.idx
}

// serveMatched runs a handler the trie matched, after restoring the request state that mux's own
// ServeHTTP would have populated.
func (rou *Router) serveMatched(w http.ResponseWriter, r *http.Request, match *mux.RouteMatch) {
	// Reinstate what mux.Router.ServeHTTP would have populated so that mux.Vars(r) (used by
	// request.go and user handlers) and the route template (used by the tracer/metrics middleware)
	// keep working.
	// len, not nil. gorilla/mux allocates a Vars map on every successful match
	// (route.go: "if match.Vars == nil { match.Vars = make(...) }"), so a route
	// with no path parameters still arrives here with an empty non-nil map --
	// and storing it cost a context node and a shallow Request copy on every
	// request to every parameter-free route, to carry nothing.
	//
	// Skipping it leaves mux.Vars(r) returning nil rather than an empty map for
	// those routes. Every read stays correct: indexing a nil map yields the zero
	// value, len is 0, and ranging over it does nothing -- which is what
	// Request.PathParam and user handlers do with it.
	if len(match.Vars) > 0 {
		r = mux.SetURLVars(r, match.Vars)
	}

	if match.Route != nil {
		if tmpl, err := match.Route.GetPathTemplate(); err == nil {
			r = withRouteTemplate(r, tmpl)
		}
	}

	// mux builds its middleware chain inside Match, guarded by MatchErr == nil, so a route that
	// matches while REPORTING an error is served WITHOUT the chain. A subrouter carrying its own
	// NotFoundHandler is that case: it reports a successful match with ErrNotFound. Running the chain
	// here would log, trace, meter and CORS-answer a request mux leaves uninstrumented — a silent
	// difference, since the status and body are identical either way.
	//
	// The mirror only needs this one guard. A subrouter's MethodNotAllowedHandler also matches
	// successfully, but mux clears ErrMethodMismatch as soon as one of the route's matchers succeeds
	// (the else arm of the matcher loop in gorilla/mux route.go), so that case arrives here with a nil
	// MatchErr and correctly DOES get the chain.
	if match.MatchErr != nil {
		match.Handler.ServeHTTP(w, r)

		return
	}

	rou.chainFor(match.Route, match.Handler).ServeHTTP(w, r)
}

// chainFor returns the composed middleware chain for a route, building it on
// first use. See the chains field for why this is memoized and why the route is
// the key.
func (rou *Router) chainFor(route *mux.Route, h http.Handler) http.Handler {
	// No route to key on: compose per request, as before. Unreachable from the
	// trie matcher, which only reaches here with a matched route.
	if route == nil {
		return composeMiddleware(rou.mws, h)
	}

	// Only routes GoFr registered itself are cached. A route reached any other way
	// -- notably one under a PathPrefix(...).Subrouter(), since Router embeds
	// mux.Router publicly -- can be matched with a handler mux built for THIS
	// request: the subrouter wraps its handler in fresh middleware each time while
	// match.Route still points at the inner route. Caching on the route alone would
	// pin the first such wrapper forever and every later request would run it
	// instead of its own.
	if _, isOwn := rou.own.Load(route); !isOwn {
		return composeMiddleware(rou.mws, h)
	}

	if v, ok := rou.chains.Load(route); ok {
		if composed, isHandler := v.(http.Handler); isHandler {
			return composed
		}
	}

	composed := composeMiddleware(rou.mws, h)

	// LoadOrStore, not Store: concurrent first requests to the same route would
	// otherwise each compose a chain and the last write would win, so "built once
	// per route" would be true of the cache but not of the work. Whichever chain
	// lands first is the one everyone uses.
	actual, _ := rou.chains.LoadOrStore(route, composed)

	rou.cached.Store(true)

	if h, ok := actual.(http.Handler); ok {
		return h
	}

	return composed
}

// Use registers mux middlewares. It records them in GoFr's own chain — so the
// trie matcher can apply them when it bypasses mux's ServeHTTP — and delegates
// to the embedded mux router, leaving the default (mux) path unchanged. It
// shadows mux.Router.Use for calls made on *Router.
func (rou *Router) Use(mwf ...mux.MiddlewareFunc) {
	rou.reportLateRegistration(len(mwf))

	rou.mws = append(rou.mws, mwf...)
	rou.Router.Use(mwf...)
}

// UseLogger gives the router somewhere to report a late middleware registration.
// It is optional: a router without one behaves identically, it just cannot say
// anything. Nothing on the request path reads it.
func (rou *Router) UseLogger(l logging.Logger) {
	rou.logger = l
}

// reportLateRegistration turns a silent misconfiguration into a logged one.
//
// Memoizing the chain per route makes registration order load-bearing in trie
// mode: a middleware registered after a route has served does not appear in that
// route's cached chain and simply never runs for it, with nothing anywhere
// saying so. GoFr registers everything before Run, so this cannot fire from
// framework code -- it fires for an application that reached the router itself,
// which is exactly the case that used to debug badly.
//
// It is an error rather than a panic because the router may already be serving
// traffic, and it is trie-only: in mux mode the chain is composed per request, so
// a late registration takes effect and there is nothing to report. The atomic
// read costs nothing against a call made a handful of times at startup.
func (rou *Router) reportLateRegistration(n int) {
	if n == 0 || !rou.useTrie || !rou.cached.Load() || rou.logger == nil {
		return
	}

	rou.logger.Errorf("%d middleware(s) registered after the router began serving: they will NOT run "+
		"for any route that has already been requested, because %s memoises each route's chain. "+
		"Register every middleware before starting the server.", n, RouterEnvVar)
}

// Matcher reports which route matcher this router uses: MatcherTrie for the
// opt-in index, MatcherMux for the default linear scan.
//
// It is exported so the server can state the active matcher at startup. The
// choice is made from the environment inside NewRouter, and an unrecognized
// GOFR_ROUTER value falls back to mux — which is indistinguishable, from the
// outside, from not setting the variable at all. Reporting the resolved matcher
// is what lets the caller tell a typo from a default.
func (rou *Router) Matcher() string {
	if rou.useTrie {
		return MatcherTrie
	}

	return MatcherMux
}

// composeMiddleware wraps h with mws so that mws[0] is the outermost layer,
// matching the order in which mux applies its middleware chain.
func composeMiddleware(mws []mux.MiddlewareFunc, h http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}

	return h
}

// isCleanPath reports whether p is already canonical — starts with "/", no
// "//", no "/.", no "/..", and no trailing slash (except the root). When
// true, path.Clean(p) == p and the surrounding normalization can be skipped.
func isCleanPath(p string) bool {
	if p == "" || p[0] != '/' {
		return false
	}

	if hasNonRootTrailingSlash(p) {
		return false
	}

	return !hasDirtySegment(p)
}

// hasNonRootTrailingSlash reports whether p ends with '/' and is longer
// than the root path.
func hasNonRootTrailingSlash(p string) bool {
	return len(p) > 1 && p[len(p)-1] == '/'
}

// hasDirtySegment reports whether p contains "//", "/.", or "/.." anywhere
// between segments — any of which means path.Clean(p) would shorten p.
func hasDirtySegment(p string) bool {
	for i := 0; i < len(p); i++ {
		if p[i] != '/' || i+1 >= len(p) {
			continue
		}

		switch p[i+1] {
		case '/':
			return true
		case '.':
			if isDotSegment(p, i+1) {
				return true
			}
		}
	}

	return false
}

// isDotSegment reports whether the dot at p[idx] starts a "." or ".."
// segment (i.e., is followed by '/' or end-of-string, optionally with a
// second '.' before that boundary).
func isDotSegment(p string, idx int) bool {
	if idx+1 == len(p) {
		return true // trailing "/."
	}

	if p[idx+1] == '/' {
		return true // "/./"
	}

	if p[idx+1] == '.' && (idx+2 == len(p) || p[idx+2] == '/') {
		return true // "/.." or "/../"
	}

	return false
}

// Add adds a new route with the given HTTP method, pattern, and handler.
//
// HTTP semconv attributes (http.method, http.route, http.status_code) are
// recorded on the request span by the framework's Tracer middleware
// directly, avoiding the per-request child span and attribute slice grow
// that an otelhttp.NewHandler wrap would add.
func (rou *Router) Add(method, pattern string, handler http.Handler) {
	rou.markOwned(rou.Router.NewRoute().Methods(method).Path(pattern).Handler(handler))
}

// markOwned records a route whose handler is the one GoFr installed and never
// changes, making it eligible for the middleware-chain cache.
//
// Every registration GoFr makes on its own router goes through here. A route
// created directly on the embedded mux.Router does not, which is the point: see
// chainFor for why such a route must not be cached.
func (rou *Router) markOwned(route *mux.Route) {
	rou.own.Store(route, struct{}{})
}

// AllowedMethods returns the sorted list of HTTP methods registered for the request's path, whatever
// the request's own method. Routes that match any method — the catch-all and static endpoints —
// declare none and are skipped, so an unregistered path yields nil.
//
// It does not walk every route. It is called for a request mux has already scanned every route to
// reject, and a second scan would double what an unknown path costs — a cost any client can incur
// at will. The route index narrows the request path to the routes registered under it, in time that
// depends on the path and not on how many routes exist, and only those are matched. A route the
// index cannot place by path (a prefix, a multi-segment pattern) is always a candidate.
//
// The narrowing cannot be replaced by reading the mismatch mux recorded during its own scan: mux
// clears ErrMethodMismatch as soon as any matcher of a later route succeeds (gorilla/mux v1.8.1
// route.go, the else arm of the matcher loop), so by the time the catch-all runs it is gone.
//
// Each candidate is matched once, against the request as it arrived: mux reports a request that
// satisfies every matcher except the method as ErrMethodMismatch (route.go, after the matcher
// loop), which is exactly "this path is registered, for other methods".
//
// The index is built once, so a route registered after the first unmatched request is not listed.
// Such a route is still served — matching does not go through here with the mux matcher — and a
// wrong method on it answers 404, as it did before this method existed.
func (rou *Router) AllowedMethods(r *http.Request) []string {
	var (
		allowed []string
		match   mux.RouteMatch
		buf     [12]*routeEntry
	)

	for _, e := range rou.index().candidates(r, buf[:0]) {
		// Reset per route: Match clears a mismatch recorded by an earlier route as soon as one of
		// this route's matchers succeeds, so a carried-over one would misreport. One variable reset,
		// rather than one declared per route, because it escapes to the heap through Match.
		match = mux.RouteMatch{}

		if !e.route.Match(r, &match) && !errors.Is(match.MatchErr, mux.ErrMethodMismatch) {
			continue
		}

		// Only now: GetMethods copies the route's method list on every call, and on a 404 no route
		// registered for a method gets this far.
		methods, err := e.route.GetMethods()
		if err != nil {
			continue
		}

		for _, m := range methods {
			if !slices.Contains(allowed, m) {
				allowed = append(allowed, m)
			}
		}
	}

	sort.Strings(allowed)

	return allowed
}

// UseMiddleware registers middlewares to the router.
func (rou *Router) UseMiddleware(mws ...Middleware) {
	middlewares := make([]mux.MiddlewareFunc, 0, len(mws))
	for _, m := range mws {
		middlewares = append(middlewares, mux.MiddlewareFunc(m))
	}

	rou.Use(middlewares...)
}

// staticFileConfig serves files from one directory through an os.Root, which is what keeps every
// request inside it.
//
// Containment used to be a string comparison on the joined path, and a string comparison cannot
// see a symlink: public/evil.txt -> ../secrets/passwords.txt is lexically inside public/ and was
// served. os.Root resolves each component, symlinks included, and refuses a path that lands outside
// the directory — with no window between the check and the open for the link to be repointed in,
// which is what a filepath.EvalSymlinks check before os.Open would leave.
//
// It also refuses an absolute symlink whose target is inside the directory, since that target is
// expressed from the filesystem root. Symlinks within a served directory have to be relative. The
// served directory itself may still be reached through a symlink (current -> releases/42); only
// links below it are resolved by the root.
//
// A root opens the served directory, and each directory on the way to a file, for reading, where
// os.Open on the joined path only had to search them. A directory the process can traverse but not
// list (mode 0711 owned by someone else) therefore no longer serves the files beneath it: the
// request answers 403, and a served directory in that state is not registered at all.
//
// The root is opened per request, not once at registration. An os.Root holds the directory it was
// opened on, so a root held for the life of the app would keep serving the old release after
// current -> releases/42 is repointed at releases/43, and a directory replaced in place would never
// be seen. Resolving directoryName on each request keeps that as it has always been.
type staticFileConfig struct {
	directoryName string
	logger        logging.Logger

	// root is the served directory opened for the request in flight; staticHandler sets it on its
	// own copy of the config.
	root *os.Root

	// errEscapes is the error os.Root reports for a path that resolves outside it. The os package
	// does not export it, so it is captured from a root at registration.
	errEscapes error
}

func (rou *Router) AddStaticFiles(logger logging.Logger, endpoint, dirName string) {
	// The route patterns below are built from endpoint verbatim, and ServeHTTP normalizes
	// incoming paths with path.Clean — so an endpoint carrying a leading or trailing slash
	// registers a pattern no request can ever match. Normalize here, where the patterns are
	// built, so a direct caller cannot register a dead route either.
	endpoint = "/" + strings.Trim(endpoint, "/")

	absDir, err := filepath.Abs(dirName)
	if err != nil {
		logger.Errorf("error in registering '%v' static endpoint, cannot resolve directory %v: %v", endpoint, dirName, err)

		return
	}

	// App.AddStaticFiles only stats the path, so a regular file passes registration. Refusing to
	// register a non-directory keeps the endpoint from serving that file verbatim at its root.
	if info, statErr := os.Stat(absDir); statErr != nil || !info.IsDir() {
		logger.Errorf("error in registering '%v' static endpoint, %v is not a directory", endpoint, absDir)

		return
	}

	root, err := os.OpenRoot(absDir)
	if err != nil {
		logger.Errorf("error in registering '%v' static endpoint, cannot open directory %v: %v", endpoint, absDir, err)

		return
	}

	errEscapes := escapeError(root)

	root.Close()

	cfg := staticFileConfig{directoryName: absDir, logger: logger, errEscapes: errEscapes}

	handler := cfg.staticHandler()

	if endpoint == "/" {
		rou.markOwned(rou.Router.NewRoute().PathPrefix(endpoint).Handler(http.StripPrefix(endpoint, handler)))

		logger.Logf("registered static files at endpoint %v from directory %v", endpoint, absDir)

		return
	}

	// The prefix route keeps its trailing separator so that a sibling endpoint sharing the prefix
	// ("/staticother" against "/static") does not match. That leaves the endpoint's own root
	// unrouted: ServeHTTP normalizes with path.Clean, which drops the trailing slash, so a request
	// for "/static/" arrives as "/static" and matches neither the prefix nor anything else. Register
	// the bare endpoint as an exact path to serve it.
	rou.markOwned(rou.Router.NewRoute().Path(endpoint).Handler(http.StripPrefix(endpoint, handler)))
	rou.markOwned(rou.Router.NewRoute().PathPrefix(endpoint + "/").Handler(http.StripPrefix(endpoint+"/", handler)))

	logger.Logf("registered static files at endpoint %v from directory %v", endpoint+"/", absDir)
}

func (staticConfig staticFileConfig) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path

		// A static endpoint only reads. Without this every method reaches the file and a DELETE is
		// answered "200, here is the content" — nothing was deleted. Restricting the routes with
		// .Methods() instead would drop the request to the catch-all and answer 404, which is its own
		// lie when GET on the same path is a 200; the check belongs here, with the Allow header
		// RFC 9110 §15.5.6 requires alongside a 405.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			staticConfig.respondWithError(w, "method not allowed on static endpoint", url, nil, http.StatusMethodNotAllowed)

			return
		}

		// Restrict direct access to openapi.json via static routes.
		// Allow access only through /.well-known/swagger or /.well-known/openapi.json.
		if staticConfig.isRestrictedFile(url) {
			staticConfig.respondWithError(w, "unauthorized attempt to access restricted file", url, nil, http.StatusForbidden)
			return
		}

		name := rootRelativeName(url)

		// A copy, so the per-request root never touches the config the handler closes over and
		// concurrent requests share.
		reqConfig := staticConfig

		root, err := os.OpenRoot(reqConfig.directoryName)
		if err != nil {
			// The served directory is gone (a release removed, a volume unmounted): a miss, not an
			// application failure. There is no root to read a custom 404 page from.
			reqConfig.respondWithFileError(w, name, err)
			return
		}

		defer root.Close()

		reqConfig.root = root

		file, info, err := reqConfig.openFile(name)
		if err != nil {
			reqConfig.respondWithFileError(w, name, err)
			return
		}

		defer file.Close()

		reqConfig.logger.Debugf("serving file: %s", file.Name())

		reqConfig.serveFile(w, r, file, info)
	})
}

// rootRelativeName turns the request path left after the endpoint prefix is stripped into a name
// relative to the served root: "" and "/" are the root itself.
func rootRelativeName(url string) string {
	name := strings.TrimPrefix(path.Clean("/"+url), "/")
	if name == "" {
		return "."
	}

	return filepath.FromSlash(name)
}

// escapeError captures the error os.Root returns for a path that leaves it, by asking for the one
// path that always does. It is compared with errors.Is so an escape can answer 404 rather than 500.
func escapeError(root *os.Root) error {
	_, err := root.Stat("..")

	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}

	return nil
}

// serveFile writes the contents of the open file to w.
//
// This is what breaks the redirects. http.FileServer answers a directory with a redirect to the
// trailing-slash form of the URL, and ".../index.html" with a redirect back to "./" — and
// ServeHTTP's path.Clean strips the slash straight back off, so the directory case sends the
// client in a circle it can never satisfy. http.ServeFile carries the same index.html rule.
// http.ServeContent has neither: it writes the bytes it is handed and never redirects. Combined
// with openFile resolving a directory to its index file, every request either gets content or
// an error.
func (staticFileConfig) serveFile(w http.ResponseWriter, r *http.Request, file *os.File, info fs.FileInfo) {
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// Checks if the file is restricted.
//
// The name is compared case-insensitively because the filesystem underneath may be. On a
// case-insensitive one — every macOS machine, and Windows — "/static/openapi.JSON" opens the very
// file an exact comparison refuses to serve, leaving the restriction only as strong as the one
// spelling it knows.
//
// Containment is not checked here: the root refuses any path that leaves the served directory.
func (staticFileConfig) isRestrictedFile(url string) bool {
	return strings.EqualFold(filepath.Base(url), DefaultSwaggerFileName)
}

// openFile opens name through the root, resolving a directory to its index file, and returns the
// open file with its FileInfo once it is known to be servable. Every lookup goes through the root,
// so a symlink that leaves the served directory fails here.
//
// The file is opened once and the checks read the open file's own FileInfo. An os.Root resolves a
// path one component at a time, so a Stat before the Open would walk the path twice; this also
// leaves no window for the file to be swapped between the check and the open.
func (staticConfig staticFileConfig) openFile(name string) (*os.File, fs.FileInfo, error) {
	file, info, err := staticConfig.openAndStat(name)
	if err != nil {
		return nil, nil, err
	}

	// A directory is served only through its index file. Handing the directory itself to
	// http.FileServer would render a listing of its contents, which a static endpoint has no
	// business disclosing. Without an index, the open reports the same not-exist error a missing
	// file gives, so the request takes the ordinary 404 path.
	if info.IsDir() {
		file.Close()

		file, info, err = staticConfig.openAndStat(filepath.Join(name, staticServerIndexFileName))
		if err != nil {
			return nil, nil, err
		}
	}

	// Only regular files are servable. Dropping http.FileServer left nothing else checking the
	// type of the resolved path, and a non-regular one reaches http.ServeContent, which writes a
	// Content-Length from its FileInfo and then no body (a directory named index.html — a 200 the
	// client reads as an unexpected EOF). fs.ErrNotExist puts it on the ordinary 404 path, the
	// same one the no-index case takes.
	if !info.Mode().IsRegular() {
		file.Close()

		return nil, nil, fs.ErrNotExist
	}

	// Ensure file has at least read (`r--`) permission.
	//
	// This asks whether anyone holds a read bit, not whether this process can read the file, so it
	// is not the authority on the question: the open is, and its EACCES covers the cases the mode
	// bits cannot (a file readable only by another user, a directory in the path that cannot be
	// traversed). Both answer 403, so this only decides which route reports it — it is kept because
	// it refuses a mode-0000 file identically whatever user the process runs as, where the open
	// alone would succeed for root.
	if info.Mode().Perm()&0444 == 0 {
		file.Close()

		return nil, nil, errReadPermissionDenied
	}

	return file, info, nil
}

// openAndStat opens name read-only through the root and stats the open file.
//
// O_NONBLOCK is what makes opening before checking the type safe: a blocking open of a fifo waits
// for a writer that never comes, and would hang the request. It changes nothing for a regular
// file or a directory, and Windows ignores it.
func (staticConfig staticFileConfig) openAndStat(name string) (*os.File, fs.FileInfo, error) {
	file, err := staticConfig.root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()

		return nil, nil, err
	}

	return file, info, nil
}

// Handles different file-related errors.
func (staticConfig staticFileConfig) respondWithFileError(w http.ResponseWriter, name string, err error) {
	// An unreadable file is the caller being refused, not the server failing: net/http's own
	// toHTTPError maps fs.ErrPermission to 403, as do nginx and Apache. Answering 500 tells a monitor
	// the application is broken when what it has is a file mode. Both routes here satisfy
	// fs.ErrPermission — a real EACCES from the open, and openFile's own
	// errReadPermissionDenied, which wraps it.
	if errors.Is(err, fs.ErrPermission) {
		staticConfig.respondWithError(w, "no read permission for file", name, err, http.StatusForbidden)
		return
	}

	// A path that resolves outside the served directory answers exactly as a missing file does, so
	// the response does not confirm that the link exists. The escape error satisfies neither
	// fs.ErrNotExist nor fs.ErrPermission, and would otherwise fall through to 500.
	if errors.Is(err, fs.ErrNotExist) || (staticConfig.errEscapes != nil && errors.Is(err, staticConfig.errEscapes)) {
		staticConfig.logger.Debugf("requested file not found: %s", name)

		// Serve custom 404.html if available. The body is written out here rather than handed to
		// http.ServeFile because the status has to be 404, not the 200 a file server writes — and
		// because ServeFile would answer a request whose own path ends in "/index.html" with a
		// redirect instead of the page. It is read through the root like any other file.
		if page, ok := staticConfig.notFoundPage(); ok {
			staticConfig.logger.Debugf("serving custom 404 page: %s", staticServerNotFoundFileName)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)

			_, _ = w.Write(page)

			return
		}

		w.WriteHeader(http.StatusNotFound)

		_, _ = w.Write([]byte("404 Not Found"))

		return
	}

	staticConfig.respondWithError(w, "error accessing file", name, err, http.StatusInternalServerError)
}

// Generic error response handler.
func (staticConfig staticFileConfig) respondWithError(w http.ResponseWriter, message, url string, err error, status int) {
	if err != nil {
		staticConfig.logger.Errorf("%s: %s, error: %v", message, url, err)
	} else {
		staticConfig.logger.Debugf("%s: %s", message, url)
	}

	w.WriteHeader(status)

	fmt.Fprintf(w, "%d %s", status, http.StatusText(status))
}

// notFoundPage returns the served directory's custom 404 page, read through the root like any
// other file. There is none when the directory itself could not be opened.
func (staticConfig staticFileConfig) notFoundPage() ([]byte, bool) {
	if staticConfig.root == nil {
		return nil, false
	}

	page, err := staticConfig.root.ReadFile(staticServerNotFoundFileName)

	return page, err == nil
}
