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

func (rou *Router) GetDefaultStaticFilesConfig() StaticFileConfig {
	staticConfig := StaticFileConfig{
		DirectoryListing: true,
		HideDotFiles:     true,
	}

	return staticConfig
}

// Static File Handling.
func (rou *Router) AddStaticFiles(endpoint string, staticConfig StaticFileConfig) {
	fileServer := http.FileServer(http.Dir(staticConfig.FileDirectory))
	rou.Router.NewRoute().PathPrefix(endpoint + "/").Handler(http.StripPrefix(endpoint, staticConfig.staticHandler(fileServer)))
}

// Check all the static handling configs.
const forbiddenBody string = "403 forbidden"

func (staticConfig StaticFileConfig) staticHandler(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path

		if staticConfig.DirectoryListing {
			staticConfig.checkDirectoryListing(w, r, url)
		}

		filePath := strings.Split(url, "/")

		fileName := filePath[len(filePath)-1]

		if staticConfig.HideDotFiles {
			staticConfig.checkDotFiles(w, fileName, url)
		}

		if len(staticConfig.ExcludeExtensions) > 0 {
			staticConfig.checkExcludedExtensions(w, fileName, url)
		}

		if len(staticConfig.ExcludeFiles) > 0 {
			staticConfig.checkExcludedFiles(w, fileName, url)
		}

		fileServer.ServeHTTP(w, r)
	})
}

func (staticConfig StaticFileConfig) checkDirectoryListing(w http.ResponseWriter, r *http.Request, url string) {
	if _, err := os.Stat(filepath.Join(staticConfig.FileDirectory, "index.html")); err != nil && strings.HasSuffix(url, "/") {
		http.NotFound(w, r)
		return
	}
}

func (staticConfig StaticFileConfig) checkDotFiles(w http.ResponseWriter, fileName, url string) {
	if _, err := os.Stat(filepath.Join(staticConfig.FileDirectory, url)); err == nil && strings.HasPrefix(fileName, ".") {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(forbiddenBody))

		return
	}

}

func (staticConfig StaticFileConfig) checkExcludedExtensions(w http.ResponseWriter, fileName, url string) {
	_, err := os.Stat(filepath.Join(staticConfig.FileDirectory, url))

	for _, ext := range staticConfig.ExcludeExtensions {
		if strings.HasSuffix(fileName, ext) && err == nil {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(forbiddenBody))

			return
		}
	}
}

func (staticConfig StaticFileConfig) checkExcludedFiles(w http.ResponseWriter, fileName, url string) {
	_, err := os.Stat(filepath.Join(staticConfig.FileDirectory, url))

	for _, file := range staticConfig.ExcludeFiles {
		if file == fileName && err == nil {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(forbiddenBody))

			return
		}
	}
}

// UseMiddleware registers middlewares to the router.
func (rou *Router) UseMiddleware(mws ...Middleware) {
	middlewares := make([]mux.MiddlewareFunc, 0, len(mws))
	for _, m := range mws {
		middlewares = append(middlewares, mux.MiddlewareFunc(m))
	}

	rou.Use(middlewares...)
}
