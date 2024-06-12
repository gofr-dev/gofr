package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gorilla/mux"
)

// Router is responsible for routing HTTP request.
type Router struct {
	mux.Router
	RegisteredRoutes *[]string
}

type StaticFileConfig struct {
	DirectoryListing  bool
	HideDotFiles      bool
	ExcludeExtensions []string
	ExcludeFiles      []string
	FileDirectory     string
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

func (router *Router) GetDefaultStaticFilesConfig() StaticFileConfig {
	config := StaticFileConfig{
		DirectoryListing: true,
		HideDotFiles:     true,
	}
	return config
}

// Static File Handling
func (rou *Router) AddStaticFiles(endpoint string, config StaticFileConfig) {
	fileServer := http.FileServer(http.Dir(config.FileDirectory))
	rou.Router.NewRoute().PathPrefix(endpoint + "/").Handler(http.StripPrefix(endpoint, staticHandler(fileServer, config)))
}

// Check all the static handling configs
func staticHandler(fileServer http.Handler, config StaticFileConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path

		const forbiddenBody string = "403 forbidden"

		if config.DirectoryListing {
			if _, err := os.Stat(filepath.Join(config.FileDirectory, "index.html")); err != nil && strings.HasSuffix(url, "/") {
				http.NotFound(w, r)
				return
			}
		}

		filePath := strings.Split(url, "/")

		fileName := filePath[len(filePath)-1]

		if config.HideDotFiles {

			if _, err := os.Stat(filepath.Join(config.FileDirectory, url)); err != nil {
				http.NotFound(w, r)
				return
			}

			if strings.HasPrefix(fileName, ".") {
				w.WriteHeader(http.StatusForbidden)
				w.Header().Set("Content-Type", "text/plain;charset=utf-8")
				w.Write([]byte(forbiddenBody))
				return
			}
		}

		if len(config.ExcludeExtensions) > 1 {

			if _, err := os.Stat(filepath.Join(config.FileDirectory, url)); err != nil {
				http.NotFound(w, r)
				return
			}

			extensions := config.ExcludeExtensions[1:]
			for _, ext := range extensions {
				if strings.HasSuffix(fileName, ext) {
					w.WriteHeader(http.StatusForbidden)
					w.Header().Set("Content-Type", "text/plain;charset=utf-8")
					w.Write([]byte(forbiddenBody))
					return
				}
			}
		}

		if len(config.ExcludeFiles) > 1 {
			if _, err := os.Stat(filepath.Join(config.FileDirectory, url)); err != nil {
				http.NotFound(w, r)
				return
			}

			excludedFiles := config.ExcludeFiles[1:]
			for _, file := range excludedFiles {
				if file == fileName {
					w.WriteHeader(http.StatusForbidden)
					w.Header().Set("Content-Type", "text/plain;charset=utf-8")
					w.Write([]byte(forbiddenBody))
					return
				}
			}
		}

		fileServer.ServeHTTP(w, r)
	})
}

// UseMiddleware registers middlewares to the router.
func (rou *Router) UseMiddleware(mws ...Middleware) {
	middlewares := make([]mux.MiddlewareFunc, 0, len(mws))
	for _, m := range mws {
		middlewares = append(middlewares, mux.MiddlewareFunc(m))
	}

	rou.Use(middlewares...)
}
