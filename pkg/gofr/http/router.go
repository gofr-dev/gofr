package http

import (
	"gofr.dev/pkg/gofr/service"
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
)

type Router struct {
	mux.Router
}

func NewRouter(c *container.Container) *Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	muxRouter.Use(
		middleware.Tracer,
		middleware.Logging(c.Logger),
		middleware.CORS(),
		middleware.Metrics(c.Metrics()),
		middleware.BasicAuthMiddleware(service.BasicAuthProvider{}),
	)

	return &Router{
		Router: *muxRouter,
	}
}

func (rou *Router) Add(method, pattern string, handler http.Handler) {
	h := otelhttp.NewHandler(handler, "gofr-router")
	rou.Router.NewRoute().Methods(method).Path(pattern).Handler(h)
}
