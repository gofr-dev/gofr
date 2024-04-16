// Package middleware provides a collection of middleware functions that handles various aspects of request handling,
// such as authentication, logging, tracing, and metrics collection.
package middleware

import (
	"net/http"
)

// APIKeyAuthMiddleware creates a middleware function that enforces API key authentication based on the provided API
// keys or a validation function.
func APIKeyAuthMiddleware(validator func(apiKey string) bool, apiKeys ...string) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWellKnown(r.URL.Path) {
				handler.ServeHTTP(w, r)
				return
			}

			authKey := r.Header.Get("X-API-KEY")
			if authKey == "" {
				http.Error(w, "Unauthorized: Authorization header missing", http.StatusUnauthorized)
				return
			}

			if !validateKey(validator, authKey, apiKeys...) {
				http.Error(w, "Unauthorized: Invalid Authorization header", http.StatusUnauthorized)
				return
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

func validateKey(validator func(apiKey string) bool, authKey string, apiKeys ...string) bool {
	if validator != nil {
		if !validator(authKey) {
			return false
		}
	} else {
		if !isPresent(authKey, apiKeys...) {
			return false
		}
	}

	return true
}
