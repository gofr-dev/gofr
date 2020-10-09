package http

import (
	"log"
	"net/http"
	"os"

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
		middleware.Recover,
		middleware.Logging(log.New(os.Stdout, "[REQ] ", log.LstdFlags)),
	)

	return &Router{
		Router: *muxRouter,
	}
}

func (rou *Router) Add(method, pattern string, handler http.Handler) {
	rou.Router.NewRoute().Methods(method).Path(pattern).Handler(handler)
}
