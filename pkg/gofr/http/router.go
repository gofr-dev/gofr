package http

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
)

// Router is responsible for routing HTTP request.
type Router struct {
	mux.Router
}

// NewRouter creates a new Router instance.
func NewRouter(c *container.Container) *Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	muxRouter.Use(
		middleware.Tracer,
		middleware.Logging(c.Logger),
		middleware.CORS(),
		middleware.Metrics(c.Metrics()),
	)

	return &Router{
		Router: *muxRouter,
	}
}

// Add adds a new route with the given HTTP method, pattern, and handler, wrapping the handler with OpenTelemetry instrumentation.
func (rou *Router) Add(method, pattern string, handler http.Handler) {
	h := otelhttp.NewHandler(handler, "gofr-router")
	rou.Router.NewRoute().Methods(method).Path(pattern).Handler(h)
}
