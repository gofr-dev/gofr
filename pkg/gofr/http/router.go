package http

import (
	"gofr.dev/pkg/gofr/container"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"gofr.dev/pkg/gofr/http/middleware"

	"github.com/gorilla/mux"
)

type Router struct {
	mux.Router
}

func NewRouter(container *container.Container) *Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	muxRouter.Use(
		middleware.Tracer,
		middleware.Logging(container.Logger),
		middleware.CORS(),
	)

	return &Router{
		Router: *muxRouter,
	}
}

func (rou *Router) Add(method, pattern string, handler http.Handler) {
	h := otelhttp.NewHandler(handler, "gofr-handler")
	rou.Router.NewRoute().Methods(method).Path(pattern).Handler(h)
}
