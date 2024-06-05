package middleware

import (
	"net/http"
	"strings"
)

const (
	allowedHeaders = "Authorization, Content-Type, x-requested-with, origin, true-client-ip, X-Correlation-ID"
)

// CORS is a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
func CORS(middlewareConfigs map[string]string, routes *[]string) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setMiddlewareHeaders(middlewareConfigs, *routes, w)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

func setMiddlewareHeaders(middlewareConfigs map[string]string, routes []string, w http.ResponseWriter) {
	routes = append(routes, "OPTIONS")

	// Set default headers
	defaultHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": strings.Join(routes, ", "),
		"Access-Control-Allow-Headers": allowedHeaders,
	}

	// Add custom headers to the default headers
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

	// Handle additional custom headers (not part of defaultHeaders)
	for header, customValue := range middlewareConfigs {
		if _, ok := defaultHeaders[header]; !ok {
			w.Header().Set(header, customValue)
		}
	}
}
