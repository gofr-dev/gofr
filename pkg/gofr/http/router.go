package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
}

func (rou *Router) AddStaticFiles(endpoint, dirName string) {
	cfg := staticFileConfig{directoryName: dirName}

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
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("500 internal server error"))

			return
		}

		// Prevent direct access to the openapi.json file via static file routes.
		// The file should only be accessible through the explicitly defined /.well-known/swagger or
		// /.well-known/openapi.json for controlled access.
		if err != nil || !strings.HasPrefix(absPath, staticConfig.directoryName) ||
			(fileName == DefaultSwaggerFileName && err == nil) {
			w.WriteHeader(http.StatusForbidden)

			_, _ = w.Write([]byte("403 forbidden"))

			return
		}

		file, err := os.Open(absPath)

		switch {
		case os.IsNotExist(err):
			// Set response status before writing to avoid GoFr overriding it
			w.WriteHeader(http.StatusNotFound)

			// Serve 404.html for missing HTML pages
			path, _ := filepath.Abs(filepath.Join(staticConfig.directoryName, staticServerNotFoundFileName))

			if _, err = os.Stat(path); !os.IsNotExist(err) {
				http.ServeFile(w, r, path)

				return
			}

			_, _ = w.Write([]byte("404 not found"))

			return

		case os.IsPermission(err):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("403 forbidden"))

			return

		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("500 internal server error"))

			return

		default:
			defer file.Close()
			fileServer.ServeHTTP(w, r)
		}
	})
}
