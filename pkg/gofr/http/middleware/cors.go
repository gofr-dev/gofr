package middleware

import (
	"net/http"
	"strings"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"
)

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
// It supports multiple allowed origins via comma-separated values in the
// Access-Control-Allow-Origin config. When multiple origins are configured,
// the middleware dynamically matches the request's Origin header and responds
// with the matched origin, adding a Vary: Origin header for correct caching.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	allowedOrigins := parseOrigins(middlewareConfigs["Access-Control-Allow-Origin"])

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
	routes = append(routes, "OPTIONS")

	// Handle Access-Control-Allow-Origin separately for dynamic matching.
	if allowedOrigins["*"] {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if allowedOrigins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}

	// Set default headers (excluding origin, handled above)
	defaultHeaders := map[string]string{
		"Access-Control-Allow-Methods": strings.Join(routes, ", "),
		"Access-Control-Allow-Headers": allowedHeaders,
	}

	for header, defaultValue := range defaultHeaders {
		if customValue, ok := middlewareConfigs[header]; ok && customValue != "" {
			if header == "Access-Control-Allow-Headers" {
				w.Header().Set(header, defaultValue+", "+customValue)
			} else {
				w.Header().Set(header, customValue)
			}
		} else {
			w.Header().Set(header, defaultValue)
		}
	}

	// Handle additional custom headers (not part of defaultHeaders or origin)
	for header, customValue := range middlewareConfigs {
		if _, ok := defaultHeaders[header]; !ok && header != "Access-Control-Allow-Origin" {
			w.Header().Set(header, customValue)
		}
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
