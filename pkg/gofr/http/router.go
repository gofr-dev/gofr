package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"gofr.dev/pkg/gofr/logging"
)

const (
	DefaultSwaggerFileName       = "openapi.json"
	staticServerNotFoundFileName = "404.html"
)

// Router is responsible for routing HTTP request.
type Router struct {
	mux.Router
	RegisteredRoutes *[]string
}

type Middleware func(handler http.Handler) http.Handler

// NewRouter creates a new Router instance.
func NewRouter() *Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	routes := make([]string, 0)
	r := &Router{
		Router:           *muxRouter,
		RegisteredRoutes: &routes,
	}

	r.Router = *muxRouter

	return r
}

// Add adds a new route with the given HTTP method, pattern, and handler, wrapping the handler with OpenTelemetry instrumentation.
func (rou *Router) Add(method, pattern string, handler http.Handler) {
	h := otelhttp.NewHandler(handler, "gofr-router")
	rou.Router.NewRoute().Methods(method).Path(pattern).Handler(h)
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

	fileServer := http.FileServer(http.Dir(cfg.directoryName))

	if endpoint != "/" {
		endpoint += "/"
	}

	rou.Router.NewRoute().PathPrefix(endpoint).Handler(http.StripPrefix(endpoint, cfg.staticHandler(fileServer)))
}

func (staticConfig staticFileConfig) staticHandler(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path

		filePath := strings.Split(url, "/")

		fileName := filePath[len(filePath)-1]

		absPath, err := filepath.Abs(filepath.Join(staticConfig.directoryName, url))
		if err != nil {
			staticConfig.logger.Errorf("failed to resolve absolute path for URL: %s, error: %v", url, err)

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("500 Internal Server Error"))

			return
		}

		// Restrict direct access to openapi.json via static routes.
		// Allow access only through /.well-known/swagger or /.well-known/openapi.json.
		if !strings.HasPrefix(absPath, staticConfig.directoryName) ||
			(fileName == DefaultSwaggerFileName) {
			staticConfig.logger.Warnf("unauthorized attempt to access restricted file: %s", url)

			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("403 Forbidden"))

			return
		}

		f, err := os.Open(absPath)

		switch {
		case os.IsNotExist(err):
			staticConfig.logger.Warnf("requested file not found: %s", absPath)
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

		case err != nil:
			staticConfig.logger.Errorf("error accessing file %s: %v", absPath, err)

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("500 Internal Server Error"))

			return

		default:
			staticConfig.logger.Debugf("serving file: %s", absPath)

			fileServer.ServeHTTP(w, r)
		}

		f.Close()
	})
}
