package middleware

import (
	"net/http"
	"slices"
)

func APIKeyAuthMiddleware(apiKeys ...string) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authKey := r.Header.Get("X-API-KEY")
			if authKey == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !slices.Contains(apiKeys, authKey) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}

func APIKeyAuthMiddlewareWithFunc(validator func(apiKey string) bool) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authKey := r.Header.Get("X-API-KEY")
			if authKey == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !validator(authKey) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
