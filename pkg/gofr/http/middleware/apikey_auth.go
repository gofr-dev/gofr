package middleware

import (
	"net/http"
)

type APIKeyAuthProvider interface {
	ValidateKey(apiKey string) bool
}

func APIKeyAuthMiddleware(authProvider APIKeyAuthProvider) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authKey := r.Header.Get("X-API-KEY")
			if authKey == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !authProvider.ValidateKey(authKey) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
