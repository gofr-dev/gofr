package http

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/vikash/gofr/pkg/gofr/logging"

	"github.com/vikash/gofr/pkg/gofr/http/middleware"

	"github.com/gorilla/mux"
)

type Router struct {
	mux.Router
}

func NewRouter() *Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	muxRouter.Use(
		middleware.Tracer,
		middleware.Logging(logging.NewLogger(logging.INFO)),
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
