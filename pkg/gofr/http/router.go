package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
	rou.Router.NewRoute().PathPrefix(endpoint + "/").Handler(http.StripPrefix(endpoint, cfg.staticHandler(fileServer)))
}

func (staticConfig staticFileConfig) staticHandler(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path

		filePath := strings.Split(url, "/")

		fileName := filePath[len(filePath)-1]

		const defaultSwaggerFileName = "openapi.json"

		if _, err := os.Stat(filepath.Clean(filepath.Join(staticConfig.directoryName, url))); fileName == defaultSwaggerFileName && err == nil {
			w.WriteHeader(http.StatusForbidden)

			_, _ = w.Write([]byte("403 forbidden"))

			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
