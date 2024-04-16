// Package middleware provides a collection of middleware functions that handles various aspects of request handling,
// such as authentication, logging, tracing, and metrics collection.
package middleware

import (
	"net/http"
	"strings"
)

// APIKeyAuthMiddleware creates a middleware function that enforces API key authentication based on the provided API
// keys or a validation function.
func APIKeyAuthMiddleware(validator func(apiKey string) bool, apiKeys ...string) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/.well-known") {
				handler.ServeHTTP(w, r)
				return
			}

			authKey := r.Header.Get("X-API-KEY")
			if authKey == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if validator != nil {
				if !validator(authKey) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			} else {
				if !isPresent(authKey, apiKeys...) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}

			handler.ServeHTTP(w, r)
		})
	}
}

func isPresent(authKey string, apiKeys ...string) bool {
	for _, key := range apiKeys {
		if authKey == key {
			return true
		}
	}

	return false
}
