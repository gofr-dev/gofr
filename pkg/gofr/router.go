package gofr

import (
	"fmt"
	"net/http"
	"strings"

	"gofr.dev/pkg"

	"github.com/gorilla/mux"
)

// Router provides support to do router specific operations like add routes, use middlewares, etc.
type Router interface {
	http.Handler

	Use(...Middleware)
	Route(method string, path string, handler Handler)
	CatchAllRoute(h Handler)
	Prefix(prefix string)
}

// Middleware are meant for implementing common functionality over the entire req/res cycle. This helps us reuse the
// code for common http things like rate-limit, log, recovery etc. Some implementations have been provided in
// gofr/middleware package.
type Middleware func(http.Handler) http.Handler

// router type abstracts mux Router. This gives us flexibility to change out underlying Router from gorilla mux to
// anything else without any problems.
type router struct {
	mux.Router

	prefix string
}

// NewRouter returns an implementation of Router interface. Right now, it uses gorilla mux as underlying Router. One can
// choose not to use this and provide their own implementation of Router - as long as the returned value adheres to our
// interface, everything will work perfectly because gofr just wants to ensure that Router has 2 capabilities: 1. to
// serve a HTTP request and 2. to support our middleware.
func NewRouter() Router {
	muxRouter := mux.NewRouter().StrictSlash(false)
	r := router{Router: *muxRouter}

	return &r
}

// Use function's sole job is to satisfy the interface requirement of gofr.Router. Since Gorilla already has a
// middleware implementation, even though our middleware type has exact same function signature, we still need to
// typecast each of the middleware to satisfy the interfaces.
func (r *router) Use(middleware ...Middleware) {
	mwf := make([]mux.MiddlewareFunc, 0, len(middleware))
	for _, m := range middleware {
		mwf = append(mwf, mux.MiddlewareFunc(m))
	}

	r.Router.Use(mwf...)
}

// Prefix prepends a prefix to all the routes, this should be called before
// specifying the routes
func (r *router) Prefix(prefix string) {
	r.prefix = prefix
}

// Route creates a new route in the Router with the given params,
// this was written to be the standard way to create a route, any additional syntactic
// sugar can be added on top of this by defining methods on the gofr struct and calling this.
func (r *router) Route(method, path string, handler Handler) {
	if r.prefix != "" && !isWellKnownEndPoint(path) {
		path = r.prefix + path
	}

	r.Router.NewRoute().Methods(method).Path(path).Handler(handler)

	if method == http.MethodGet {
		r.Router.NewRoute().Methods(http.MethodHead).Path(path).Handler(handler)
	}
}

// String returns all the route and methods registered for the server
//
//nolint:gocognit // reduces readability
func (r *router) String() string {
	var availableRoutes []string

	_ = r.Router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		t, err := route.GetPathTemplate()
		if err != nil {
			return err
		}

		if t != "/" {
			t = strings.TrimSuffix(t, "/")
		}

		methods, err := route.GetMethods()
		if err != nil || methods[0] == "" {
			return err
		}

		routeStr := fmt.Sprintf("%s %s ", methods[0], t)

		if !contains(availableRoutes, routeStr) {
			availableRoutes = append(availableRoutes, routeStr)
		}

		return nil
	})

	return strings.Join(availableRoutes, "")
}

// CatchAllRoute assigns the provided Handler function to handle any request path prefix ("/")
func (r *router) CatchAllRoute(h Handler) {
	r.Router.PathPrefix("/").Handler(h)
}

func contains(elem []string, key string) bool {
	for _, v := range elem {
		if v == key {
			return true
		}
	}

	return false
}

// isWellKnownEndPoint checks whether the given path is a well-known endpoint
func isWellKnownEndPoint(path string) bool {
	return path == pkg.PathHealthCheck || path == pkg.PathHeartBeat || path == pkg.PathOpenAPI ||
		path == pkg.PathSwagger || path == pkg.PathSwaggerWithPathParam
}
