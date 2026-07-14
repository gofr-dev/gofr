package gofr

import (
	"reflect"
	"strconv"
	"time"
)

// RouteOption configures how a route is registered.
type RouteOption func(*routeConfig)

type routeConfig struct {
	inputType reflect.Type
}

// WithInput declares the request body type T for a route so that, when the route is exposed as an
// MCP tool via EnableMCP, its input schema describes the body's fields and types instead of only the
// path parameters.
func WithInput[T any]() RouteOption {
	return func(c *routeConfig) { c.inputType = reflect.TypeFor[T]() }
}

// GET adds a Handler for HTTP GET method for a route pattern.
func (a *App) GET(pattern string, handler Handler, opts ...RouteOption) {
	a.add("GET", pattern, handler, opts...)
}

// PUT adds a Handler for HTTP PUT method for a route pattern.
func (a *App) PUT(pattern string, handler Handler, opts ...RouteOption) {
	a.add("PUT", pattern, handler, opts...)
}

// POST adds a Handler for HTTP POST method for a route pattern.
func (a *App) POST(pattern string, handler Handler, opts ...RouteOption) {
	a.add("POST", pattern, handler, opts...)
}

// DELETE adds a Handler for HTTP DELETE method for a route pattern.
func (a *App) DELETE(pattern string, handler Handler, opts ...RouteOption) {
	a.add("DELETE", pattern, handler, opts...)
}

// PATCH adds a Handler for HTTP PATCH method for a route pattern.
func (a *App) PATCH(pattern string, handler Handler, opts ...RouteOption) {
	a.add("PATCH", pattern, handler, opts...)
}

func (a *App) add(method, pattern string, h Handler, opts ...RouteOption) {
	if !a.httpRegistered && !isPortAvailable(a.httpServer.port) {
		a.container.Logger.Fatalf("http port %d is blocked or unreachable", a.httpServer.port)
	}

	a.httpRegistered = true

	reqTimeout, err := strconv.Atoi(a.Config.Get("REQUEST_TIMEOUT"))
	if (err != nil && a.Config.Get("REQUEST_TIMEOUT") != "") || reqTimeout < 0 {
		reqTimeout = 0
	}

	a.registerRouteConfig(method, pattern, opts)

	a.httpServer.router.Add(method, pattern, handler{
		function:       h,
		container:      a.container,
		requestTimeout: time.Duration(reqTimeout) * time.Second,
	})
}

func (a *App) registerRouteConfig(method, pattern string, opts []RouteOption) {
	if len(opts) == 0 {
		return
	}

	var cfg routeConfig
	for _, o := range opts {
		o(&cfg)
	}

	if cfg.inputType != nil {
		if a.routeInputs == nil {
			a.routeInputs = make(map[string]reflect.Type)
		}

		a.routeInputs[method+" "+pattern] = cfg.inputType
	}
}

// AddRESTHandlers creates and registers CRUD routes for the given struct, the struct should always be passed by reference.
func (a *App) AddRESTHandlers(object any) error {
	cfg, err := scanEntity(object)
	if err != nil {
		a.container.Logger.Errorf("%v", err)
		return err
	}

	a.registerCRUDHandlers(cfg, object)

	return nil
}
