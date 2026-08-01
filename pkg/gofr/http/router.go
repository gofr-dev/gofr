package http

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/logging"
)

const (
	DefaultSwaggerFileName       = "openapi.json"
	staticServerNotFoundFileName = "404.html"
	staticServerIndexFileName    = "index.html"
)

var errReadPermissionDenied = fmt.Errorf("file does not have read permission")

// Router is responsible for routing HTTP request.
type Router struct {
	mux.Router
	RegisteredRoutes *[]string
}

type Middleware func(handler http.Handler) http.Handler

// NewRouter creates a new Router instance.
func NewRouter() *Router {
	muxRouter := mux.NewRouter().StrictSlash(false).SkipClean(true)
	routes := make([]string, 0)
	r := &Router{
		Router:           *muxRouter,
		RegisteredRoutes: &routes,
	}

	r.Router = *muxRouter

	return r
}

// ServeHTTP implements [http.Handler] interface with path normalization.
func (rou *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Normalize the path before routing to handle double slashes
	originalPath := r.URL.Path

	// Fast path: the vast majority of incoming paths are already canonical
	// ("/users/42", "/api/v1/things"). Skip the path.Clean + string ops in
	// that case so they only run for inputs that actually need normalizing.
	if !isCleanPath(originalPath) {
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

	// Delegate to the underlying Gorilla Mux router
	rou.Router.ServeHTTP(w, r)
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
	cfg := staticFileConfig{directoryName: dirName, logger: logger}

	handler := cfg.staticHandler()

	if endpoint == "/" {
		rou.Router.NewRoute().PathPrefix(endpoint).Handler(http.StripPrefix(endpoint, handler))

		logger.Logf("registered static files at endpoint %v from directory %v", endpoint, dirName)

		return
	}

	// The prefix route keeps its trailing separator so that a sibling endpoint sharing the prefix
	// ("/staticother" against "/static") does not match. That leaves the endpoint's own root
	// unrouted: ServeHTTP normalizes with path.Clean, which drops the trailing slash, so a request
	// for "/static/" arrives as "/static" and matches neither the prefix nor anything else. Register
	// the bare endpoint as an exact path to serve it.
	rou.Router.NewRoute().Path(endpoint).Handler(http.StripPrefix(endpoint, handler))
	rou.Router.NewRoute().PathPrefix(endpoint + "/").Handler(http.StripPrefix(endpoint+"/", handler))

	logger.Logf("registered static files at endpoint %v from directory %v", endpoint+"/", dirName)
}

func (staticConfig staticFileConfig) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path

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
			staticConfig.respondWithFileError(w, r, absPath, err)
			return
		}

		// validateFile has already resolved a directory to its index file, so what is served here
		// is always a plain file. That is what breaks the redirect loop: http.FileServer answers a
		// directory with a redirect to the trailing-slash form of the URL, and ServeHTTP's
		// path.Clean strips that slash straight back off, so the client is sent in a circle it can
		// never satisfy. Serving the index file itself never reaches that redirect.
		staticConfig.logger.Debugf("serving file: %s", resolvedPath)

		http.ServeFile(w, r, resolvedPath)
	})
}

// Checks if the file is restricted.
func (staticConfig staticFileConfig) isRestrictedFile(url, absPath string) bool {
	fileName := filepath.Base(url)

	return !staticConfig.isWithinDirectory(absPath) || fileName == DefaultSwaggerFileName
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

	// Ensure file has at least read (`r--`) permission
	if fileInfo.Mode().Perm()&0444 == 0 {
		return "", errReadPermissionDenied
	}

	return absPath, nil
}

// Handles different file-related errors.
func (staticConfig staticFileConfig) respondWithFileError(w http.ResponseWriter, r *http.Request, absPath string, err error) {
	if os.IsNotExist(err) {
		staticConfig.logger.Debugf("requested file not found: %s", absPath)

		w.WriteHeader(http.StatusNotFound)

		// Serve custom 404.html if available
		notFoundPath, _ := filepath.Abs(filepath.Join(staticConfig.directoryName, staticServerNotFoundFileName))
		if _, err = os.Stat(notFoundPath); err == nil {
			staticConfig.logger.Debugf("serving custom 404 page: %s", notFoundPath)

			http.ServeFile(w, r, notFoundPath)

			return
		}

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
