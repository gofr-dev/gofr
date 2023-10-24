package gofr

import (
	"regexp"
)

type cmdRoute struct {
	pattern string
	handler Handler
}

// NewCMDRouter creates and returns a new instance of the CMDRouter type
func NewCMDRouter() CMDRouter {
	return CMDRouter{}
}

type CMDRouter struct {
	routes []cmdRoute
}

// AddRoute method, belonging to the CMDRouter type, allows the addition of a new route pattern and its corresponding handler function.
// It appends a new route entry to the router's internal list of routes, enabling dynamic routing functionality.
func (r *CMDRouter) AddRoute(pattern string, handler Handler) {
	r.routes = append(r.routes, cmdRoute{pattern: pattern, handler: handler})
}

func (r *CMDRouter) handler(path string) Handler {
	for _, route := range r.routes {
		if r.match(route.pattern, path) {
			return route.handler
		}
	}

	return nil
}

func (r *CMDRouter) match(pattern, route string) bool {
	re := regexp.MustCompile(pattern)
	return re.MatchString(route)
}
