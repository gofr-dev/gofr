package gofr

import (
	"strconv"
	"time"
)

// GET adds a Handler for HTTP GET method for a route pattern.
func (a *App) GET(pattern string, handler Handler) {
	a.add("GET", pattern, handler)
}

// PUT adds a Handler for HTTP PUT method for a route pattern.
func (a *App) PUT(pattern string, handler Handler) {
	a.add("PUT", pattern, handler)
}

// POST adds a Handler for HTTP POST method for a route pattern.
func (a *App) POST(pattern string, handler Handler) {
	a.add("POST", pattern, handler)
}

// DELETE adds a Handler for HTTP DELETE method for a route pattern.
func (a *App) DELETE(pattern string, handler Handler) {
	a.add("DELETE", pattern, handler)
}

// PATCH adds a Handler for HTTP PATCH method for a route pattern.
func (a *App) PATCH(pattern string, handler Handler) {
	a.add("PATCH", pattern, handler)
}

func (a *App) add(method, pattern string, h Handler) {
	if !a.httpRegistered && !isPortAvailable(a.httpServer.port) {
		a.container.Logger.Fatalf("http port %d is blocked or unreachable", a.httpServer.port)
	}

	a.httpRegistered = true

	reqTimeout, err := strconv.Atoi(a.Config.Get("REQUEST_TIMEOUT"))
	if (err != nil && a.Config.Get("REQUEST_TIMEOUT") != "") || reqTimeout < 0 {
		reqTimeout = 0
	}

	a.httpServer.router.Add(method, pattern, handler{
		function:       h,
		container:      a.container,
		requestTimeout: time.Duration(reqTimeout) * time.Second,
		mcpLearner:     a.mcpLearner,
		method:         method,
		path:           pattern,
	})

	// Seed the MCP manifest with the route — keeps tools/list complete
	// even before any traffic has flowed through the route.
	if a.mcpLearner != nil {
		a.mcpLearner.Register(method, pattern, mcpPathParams(pattern))
	}
}

// mcpPathParams extracts {name} placeholders from a mux path pattern.
// Kept here rather than in the mcp package to avoid pulling regex
// helpers across an extra import boundary on every route registration.
func mcpPathParams(pattern string) []string {
	var params []string

	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '{' {
			continue
		}

		j := i + 1

		for j < len(pattern) && pattern[j] != '}' && pattern[j] != ':' {
			j++
		}

		if j > i+1 {
			params = append(params, pattern[i+1:j])
		}

		// Advance past the closing brace if there's a regex constraint.
		for j < len(pattern) && pattern[j] != '}' {
			j++
		}

		i = j
	}

	return params
}

// AddRESTHandlers creates and registers CRUD routes for the given struct, the struct should always be passed by reference.
func (a *App) AddRESTHandlers(object any) error {
	cfg, err := scanEntity(object)
	if err != nil {
		a.container.Logger.Errorf(err.Error())
		return err
	}

	a.registerCRUDHandlers(cfg, object)

	return nil
}
