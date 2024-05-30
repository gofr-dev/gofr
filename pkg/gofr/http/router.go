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
	RegisteredRoutes *[]string
}

type Middleware func(handler http.Handler) http.Handler

// NewRouter creates a new Router instance.
func NewRouter(c *container.Container, middlewareConfigs map[string]string) *Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	routes := make([]string, 0)
	r := &Router{
		Router:           *muxRouter,
		RegisteredRoutes: &routes,
	}

	muxRouter.Use(
		middleware.Tracer,
		middleware.Logging(c.Logger),
		middleware.CORS(middlewareConfigs, r.RegisteredRoutes),
		middleware.Metrics(c.Metrics()),
	)

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
