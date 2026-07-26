package middleware

import (
	"net/http"
	"strings"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"

	headerAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	headerAccessControlAllowMethods = "Access-Control-Allow-Methods"
	headerAccessControlAllowHeaders = "Access-Control-Allow-Headers"
)

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
// It supports multiple allowed origins via comma-separated values in the
// Access-Control-Allow-Origin config. When multiple origins are configured,
// the middleware dynamically matches the request's Origin header and responds
// with the matched origin, adding a Vary: Origin header for correct caching.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	allowedOrigins := parseOrigins(middlewareConfigs[headerAccessControlAllowOrigin])

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setMiddlewareHeaders(middlewareConfigs, *routes, w, r.Header.Get("Origin"), allowedOrigins)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

func setMiddlewareHeaders(middlewareConfigs map[string]string, routes []string,
	w http.ResponseWriter, origin string, allowedOrigins map[string]bool,
) {
	// Handle Access-Control-Allow-Origin separately for dynamic matching.
	if allowedOrigins["*"] {
		w.Header().Set(headerAccessControlAllowOrigin, "*")
	} else if allowedOrigins[origin] {
		w.Header().Set(headerAccessControlAllowOrigin, origin)
		w.Header().Add("Vary", "Origin")
	}

	// Walk the configuration once, classifying each entry by its CANONICAL
	// header name. Header names are case-insensitive and Header.Set canonicalizes
	// whatever it is given, so matching on the raw key would let a
	// differently-cased spelling reach a header it was never checked against —
	// replacing the origin negotiated above, or overwriting the default
	// allow-list instead of extending it.
	var customMethods, customHeaders string

	for key, value := range middlewareConfigs {
		switch http.CanonicalHeaderKey(key) {
		case headerAccessControlAllowOrigin:
			// Always negotiated against the configured allow-list above; never
			// overridable from here.
		case headerAccessControlAllowMethods:
			customMethods = value
		case headerAccessControlAllowHeaders:
			customHeaders = value
		default:
			w.Header().Set(key, value)
		}
	}

	// A configured method list replaces the derived one; configured headers
	// EXTEND the defaults rather than replacing them, so the framework's own
	// required headers (Authorization, Content-Type, X-Correlation-ID, ...)
	// cannot be dropped by adding one of your own.
	if customMethods == "" {
		customMethods = joinAllowedMethods(routes)
	}

	allowHeaderValue := allowedHeaders
	if customHeaders != "" {
		allowHeaderValue = allowedHeaders + ", " + customHeaders
	}

	w.Header().Set(headerAccessControlAllowMethods, customMethods)
	w.Header().Set(headerAccessControlAllowHeaders, allowHeaderValue)
}

// joinAllowedMethods renders the Access-Control-Allow-Methods value: the
// registered routes plus OPTIONS.
//
// It deliberately does not append to routes. That slice shares its backing
// array with the caller's (the router's RegisteredRoutes), so appending in
// place writes "OPTIONS" over the caller's next element whenever cap > len.
func joinAllowedMethods(routes []string) string {
	if len(routes) == 0 {
		return http.MethodOptions
	}

	return strings.Join(routes, ", ") + ", " + http.MethodOptions
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
