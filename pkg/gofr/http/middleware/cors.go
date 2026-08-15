package middleware

import (
	"net/http"
	"net/textproto"
	"strings"
	"sync"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"

	headerAccessControlAllowMethods = "Access-Control-Allow-Methods"
	headerAccessControlAllowHeaders = "Access-Control-Allow-Headers"
	headerAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
)

// Canonical keys and the shared wildcard value, resolved once at package level
// so no response pays for canonicalization or a value-slice allocation.
//
//nolint:gochecknoglobals // immutable, process-wide header constants.
var (
	canonicalAllowOrigin  = textproto.CanonicalMIMEHeaderKey(headerAccessControlAllowOrigin)
	canonicalAllowMethods = textproto.CanonicalMIMEHeaderKey(headerAccessControlAllowMethods)
	wildcardOrigin        = []string{"*"}
)

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
// It supports multiple allowed origins via comma-separated values in the
// Access-Control-Allow-Origin config. When multiple origins are configured,
// the middleware dynamically matches the request's Origin header and responds
// with the matched origin, adding a Vary: Origin header for correct caching.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	allowedOrigins := parseOrigins(middlewareConfigs[headerAccessControlAllowOrigin])

	// Every header this middleware writes, apart from Allow-Origin, is a pure
	// function of the configuration and the registered route set. Both are fixed
	// before the first request, yet they were recomputed on every one: a map
	// literal, a strings.Join over the route list, and an append into the
	// caller's slice. They are built once here instead, on the first request,
	// because the route list is still being populated when CORS is constructed.
	var (
		once    sync.Once
		fixed   []headerValue
		methods []string
	)

	build := func() {
		fixed, methods = buildFixedHeaders(middlewareConfigs, *routes)
	}

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			once.Do(build)
			setMiddlewareHeaders(w, r.Header.Get("Origin"), allowedOrigins, fixed, methods)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

// headerValue is a precomputed response header, held in the shape net/http
// stores it: a canonical key and a one-element value slice.
//
// Writing it with Header.Set would canonicalize the key and allocate a fresh
// []string on every response. Assigning the map entry directly does neither.
// Sharing the slice across responses is safe because it has len == cap == 1, so
// a later Header.Add appends into a NEW array rather than mutating this one --
// pinned by TestSharedHeaderValueSurvivesAdd.
type headerValue struct {
	key   string
	value []string
}

// buildFixedHeaders computes every header whose value cannot change once routes
// are registered. It returns them alongside the Access-Control-Allow-Methods
// value, which is kept separate only because it is the one derived from routes.
//
// The method list is joined without appending to the caller's slice: that slice
// is the router's RegisteredRoutes, and appending in place would write into it
// whenever its capacity exceeded its length.
func buildFixedHeaders(middlewareConfigs map[string]string, routes []string) (fixed []headerValue, methods []string) {
	allowMethods := joinAllowedMethods(routes)
	if custom, ok := middlewareConfigs[headerAccessControlAllowMethods]; ok && custom != "" {
		allowMethods = custom
	}

	methods = []string{allowMethods}

	allowHeaders := allowedHeaders
	if custom, ok := middlewareConfigs[headerAccessControlAllowHeaders]; ok && custom != "" {
		allowHeaders = allowedHeaders + ", " + custom
	}

	fixed = append(fixed, headerValue{
		key:   textproto.CanonicalMIMEHeaderKey(headerAccessControlAllowHeaders),
		value: []string{allowHeaders},
	})

	// Any other configured header is passed through unchanged. Allow-Origin is
	// excluded because it is negotiated per request against the allow-list.
	for header, value := range middlewareConfigs {
		switch header {
		case headerAccessControlAllowOrigin, headerAccessControlAllowMethods, headerAccessControlAllowHeaders:
			continue
		default:
			fixed = append(fixed, headerValue{
				key:   textproto.CanonicalMIMEHeaderKey(header),
				value: []string{value},
			})
		}
	}

	return fixed, methods
}

// joinAllowedMethods renders Access-Control-Allow-Methods: the registered routes
// plus OPTIONS.
func joinAllowedMethods(routes []string) string {
	if len(routes) == 0 {
		return http.MethodOptions
	}

	return strings.Join(routes, ", ") + ", " + http.MethodOptions
}

// setMiddlewareHeaders writes the CORS headers for one response. Only
// Allow-Origin depends on the request; everything else was built once.
func setMiddlewareHeaders(w http.ResponseWriter, origin string, allowedOrigins map[string]bool,
	fixed []headerValue, methods []string) {
	header := w.Header()

	if allowedOrigins["*"] {
		header[canonicalAllowOrigin] = wildcardOrigin
	} else if allowedOrigins[origin] {
		// The value varies per request, so this one cannot be shared.
		header.Set(headerAccessControlAllowOrigin, origin)
		header.Add("Vary", "Origin")
	}

	header[canonicalAllowMethods] = methods

	for _, h := range fixed {
		header[h.key] = h.value
	}
}

// parseOrigins splits a comma-separated origin string into a set.
// An empty string defaults to wildcard ("*").
func parseOrigins(raw string) map[string]bool {
	if raw == "" {
		return map[string]bool{"*": true}
	}

	origins := make(map[string]bool)

	for _, o := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins[trimmed] = true
		}
	}

	if len(origins) == 0 {
		return map[string]bool{"*": true}
	}

	return origins
}
