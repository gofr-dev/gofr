package http

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

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
	rou.buildIdx.Do(func() {
		idx := newRouteIndex()
		idx.build(&rou.Router)
		rou.idx = idx
	})

	var match mux.RouteMatch

	if rou.idx.match(r, &match) && match.Handler != nil {
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

// serveMatched runs a handler the trie matched, after restoring the request state that mux's own
// ServeHTTP would have populated.
func (rou *Router) serveMatched(w http.ResponseWriter, r *http.Request, match *mux.RouteMatch) {
	// Reinstate what mux.Router.ServeHTTP would have populated so that mux.Vars(r) (used by
	// request.go and user handlers) and the route template (used by the tracer/metrics middleware)
	// keep working.
	if match.Vars != nil {
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

	composeMiddleware(rou.mws, match.Handler).ServeHTTP(w, r)
}

// Use registers mux middlewares. It records them in GoFr's own chain — so the
// trie matcher can apply them when it bypasses mux's ServeHTTP — and delegates
// to the embedded mux router, leaving the default (mux) path unchanged. It
// shadows mux.Router.Use for calls made on *Router.
func (rou *Router) Use(mwf ...mux.MiddlewareFunc) {
	rou.mws = append(rou.mws, mwf...)
	rou.Router.Use(mwf...)
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
	rou.Router.NewRoute().Methods(method).Path(pattern).Handler(handler)
}

// UseMiddleware registers middlewares to the router.
func (rou *Router) UseMiddleware(mws ...Middleware) {
	middlewares := make([]mux.MiddlewareFunc, 0, len(mws))
	for _, m := range mws {
		middlewares = append(middlewares, mux.MiddlewareFunc(m))
	}

	rou.Use(middlewares...)
}

type staticFileConfig struct {
	directoryName string
	logger        logging.Logger
}

func (rou *Router) AddStaticFiles(logger logging.Logger, endpoint, dirName string) {
	// staticHandler resolves each request to an absolute, cleaned path and the containment check
	// compares it against directoryName as a string, so the two have to be in the same form.
	// Resolving once here covers a relative name and an absolute one carrying a trailing separator
	// — both of which would otherwise match nothing and answer every request with 403.
	absDir, err := filepath.Abs(dirName)
	if err != nil {
		logger.Errorf("error in registering '%v' static endpoint, cannot resolve directory %v: %v", endpoint, dirName, err)

		return
	}

	// App.AddStaticFiles only stats the path, so a regular file passes registration. The containment
	// check admits the served path itself now that the endpoint root has to resolve to it, which
	// would turn that misconfiguration into the file being served verbatim at the endpoint. Refusing
	// to register a non-directory keeps that guard closed where the prefix comparison used to.
	if info, statErr := os.Stat(absDir); statErr != nil || !info.IsDir() {
		logger.Errorf("error in registering '%v' static endpoint, %v is not a directory", endpoint, absDir)

		return
	}

	cfg := staticFileConfig{directoryName: absDir, logger: logger}

	handler := cfg.staticHandler()

	if endpoint == "/" {
		rou.Router.NewRoute().PathPrefix(endpoint).Handler(http.StripPrefix(endpoint, handler))

		logger.Logf("registered static files at endpoint %v from directory %v", endpoint, absDir)

		return
	}

	// The prefix route keeps its trailing separator so that a sibling endpoint sharing the prefix
	// ("/staticother" against "/static") does not match. That leaves the endpoint's own root
	// unrouted: ServeHTTP normalizes with path.Clean, which drops the trailing slash, so a request
	// for "/static/" arrives as "/static" and matches neither the prefix nor anything else. Register
	// the bare endpoint as an exact path to serve it.
	rou.Router.NewRoute().Path(endpoint).Handler(http.StripPrefix(endpoint, handler))
	rou.Router.NewRoute().PathPrefix(endpoint + "/").Handler(http.StripPrefix(endpoint+"/", handler))

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

		absPath, err := filepath.Abs(filepath.Join(staticConfig.directoryName, url))
		if err != nil {
			staticConfig.respondWithError(w, "failed to resolve absolute path", url, err, http.StatusInternalServerError)
			return
		}

		// Restrict direct access to openapi.json via static routes.
		// Allow access only through /.well-known/swagger or /.well-known/openapi.json.
		if staticConfig.isRestrictedFile(url, absPath) {
			staticConfig.respondWithError(w, "unauthorized attempt to access restricted file", url, nil, http.StatusForbidden)
			return
		}

		resolvedPath, err := staticConfig.validateFile(absPath)
		if err != nil {
			staticConfig.respondWithFileError(w, absPath, err)
			return
		}

		staticConfig.logger.Debugf("serving file: %s", resolvedPath)

		if err := staticConfig.serveFile(w, r, resolvedPath); err != nil {
			staticConfig.respondWithFileError(w, resolvedPath, err)
		}
	})
}

// serveFile writes the contents of path to w.
//
// This is what breaks the redirects. http.FileServer answers a directory with a redirect to the
// trailing-slash form of the URL, and ".../index.html" with a redirect back to "./" — and
// ServeHTTP's path.Clean strips the slash straight back off, so the directory case sends the
// client in a circle it can never satisfy. http.ServeFile carries the same index.html rule.
// http.ServeContent has neither: it writes the bytes it is handed and never redirects. Combined
// with validateFile resolving a directory to its index file, every request either gets content or
// an error.
//
// It also re-uses the open file's own FileInfo rather than stat-ing the path a second time.
func (staticFileConfig) serveFile(w http.ResponseWriter, r *http.Request, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	http.ServeContent(w, r, filepath.Base(filePath), fileInfo.ModTime(), file)

	return nil
}

// Checks if the file is restricted.
//
// The name is compared case-insensitively because the filesystem underneath may be. On a
// case-insensitive one — every macOS machine, and Windows — "/static/openapi.JSON" opens the very
// file an exact comparison refuses to serve, leaving the restriction only as strong as the one
// spelling it knows.
func (staticConfig staticFileConfig) isRestrictedFile(url, absPath string) bool {
	fileName := filepath.Base(url)

	return !staticConfig.isWithinDirectory(absPath) || strings.EqualFold(fileName, DefaultSwaggerFileName)
}

// isWithinDirectory reports whether absPath is the served directory itself or a path inside it.
//
// The trailing separator is what keeps a sibling directory with a shared prefix out — /app/public
// must not admit /app/publicother. But it also excludes the directory's own path, which carries no
// trailing separator, and that is the path a request for the endpoint root resolves to. Comparing
// against the directory as well lets the root be served without letting a sibling in.
func (staticConfig staticFileConfig) isWithinDirectory(absPath string) bool {
	return absPath == staticConfig.directoryName ||
		strings.HasPrefix(absPath, staticConfig.directoryName+string(os.PathSeparator))
}

// Validates file existence and permissions, and resolves a directory to the file that represents
// it. The returned path is absPath for an ordinary file, and absPath's index file for a directory.
func (staticFileConfig) validateFile(absPath string) (resolvedPath string, err error) {
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}

	// A directory is served only through its index file. Handing the directory itself to
	// http.FileServer would render a listing of its contents, which a static endpoint has no
	// business disclosing. Without an index, os.Stat reports the same not-exist error a missing
	// file gives, so the request takes the ordinary 404 path.
	if fileInfo.IsDir() {
		absPath = filepath.Join(absPath, staticServerIndexFileName)

		fileInfo, err = os.Stat(absPath)
		if err != nil {
			return "", err
		}
	}

	// Only regular files are servable. Dropping http.FileServer left nothing else checking the
	// type of the resolved path, and a non-regular one reaches http.ServeContent, which writes a
	// Content-Length from its FileInfo and then no body (a directory named index.html — a 200 the
	// client reads as an unexpected EOF), or blocks forever in os.Open (a fifo). fs.ErrNotExist
	// puts both on the ordinary 404 path, the same one the no-index case takes.
	if !fileInfo.Mode().IsRegular() {
		return "", fs.ErrNotExist
	}

	// Ensure file has at least read (`r--`) permission.
	//
	// This asks whether anyone holds a read bit, not whether this process can read the file, so it
	// is not the authority on the question: os.Open in serveFile is, and its EACCES covers the cases
	// the mode bits cannot (a file readable only by another user, a directory in the path that
	// cannot be traversed). Both now answer 403, so this only decides which route reports it — it is
	// kept because it refuses a mode-0000 file identically whatever user the process runs as, where
	// os.Open alone would serve it to root.
	if fileInfo.Mode().Perm()&0444 == 0 {
		return "", errReadPermissionDenied
	}

	return absPath, nil
}

// Handles different file-related errors.
func (staticConfig staticFileConfig) respondWithFileError(w http.ResponseWriter, absPath string, err error) {
	// An unreadable file is the caller being refused, not the server failing: net/http's own
	// toHTTPError maps fs.ErrPermission to 403, as do nginx and Apache. Answering 500 tells a monitor
	// the application is broken when what it has is a file mode. Both routes here satisfy
	// fs.ErrPermission — a real EACCES from os.Stat/os.Open, and validateFile's own
	// errReadPermissionDenied, which wraps it.
	if errors.Is(err, fs.ErrPermission) {
		staticConfig.respondWithError(w, "no read permission for file", absPath, err, http.StatusForbidden)
		return
	}

	if errors.Is(err, fs.ErrNotExist) {
		staticConfig.logger.Debugf("requested file not found: %s", absPath)

		// Serve custom 404.html if available. The body is written out here rather than handed to
		// http.ServeFile because the status has to be 404, not the 200 a file server writes — and
		// because ServeFile would answer a request whose own path ends in "/index.html" with a
		// redirect instead of the page.
		notFoundPath, _ := filepath.Abs(filepath.Join(staticConfig.directoryName, staticServerNotFoundFileName))

		if page, readErr := os.ReadFile(notFoundPath); readErr == nil {
			staticConfig.logger.Debugf("serving custom 404 page: %s", notFoundPath)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)

			_, _ = w.Write(page)

			return
		}

		w.WriteHeader(http.StatusNotFound)

		_, _ = w.Write([]byte("404 Not Found"))

		return
	}

	staticConfig.respondWithError(w, "error accessing file", absPath, err, http.StatusInternalServerError)
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
