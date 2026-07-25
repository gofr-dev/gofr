package middleware

import (
	"net/http"
	"strings"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"

	headerAllowOrigin  = "Access-Control-Allow-Origin"
	headerAllowMethods = "Access-Control-Allow-Methods"
	headerAllowHeaders = "Access-Control-Allow-Headers"
)

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
// It supports multiple allowed origins via comma-separated values in the
// Access-Control-Allow-Origin config. When multiple origins are configured,
// the middleware dynamically matches the request's Origin header and responds
// with the matched origin, adding a Vary: Origin header for correct caching.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	allowedOrigins := parseOrigins(middlewareConfigs[headerAllowOrigin])

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
		w.Header().Set(headerAllowOrigin, "*")
	} else if allowedOrigins[origin] {
		w.Header().Set(headerAllowOrigin, origin)
		w.Header().Add("Vary", "Origin")
	}

	// Set default headers (excluding origin, handled above)
	defaultHeaders := map[string]string{
		headerAllowMethods: joinAllowedMethods(routes),
		headerAllowHeaders: allowedHeaders,
	}

	for header, defaultValue := range defaultHeaders {
		if customValue, ok := middlewareConfigs[header]; ok && customValue != "" {
			if header == headerAllowHeaders {
				w.Header().Set(header, defaultValue+", "+customValue)
			} else {
				w.Header().Set(header, customValue)
			}
		} else {
			w.Header().Set(header, defaultValue)
		}
	}

	// Handle additional custom headers (not part of defaultHeaders or origin).
	//
	// The origin is compared on its canonical header form: HTTP header names are
	// case-insensitive and Header.Set canonicalizes whatever it is given, so a
	// differently-cased spelling ("access-control-allow-origin") would otherwise
	// pass this guard and still land on Access-Control-Allow-Origin — silently
	// replacing the origin negotiated against the configured allow-list above.
	for header, customValue := range middlewareConfigs {
		if _, ok := defaultHeaders[header]; ok {
			continue
		}

		if http.CanonicalHeaderKey(header) == headerAllowOrigin {
			continue
		}

		w.Header().Set(header, customValue)
	}
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
