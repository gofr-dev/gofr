package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"gofr.dev/pkg/gofr/container"
)

// BasicAuthProvider represents a basic authentication provider.
type BasicAuthProvider struct {
	Users                       map[string]string
	ValidateFunc                func(username, password string) bool
	ValidateFuncWithDatasources func(c *container.Container, username, password string) bool
	Container                   *container.Container
}

// BasicAuthMiddleware creates a middleware function that enforces basic authentication using the provided BasicAuthProvider.
func BasicAuthMiddleware(basicAuthProvider BasicAuthProvider) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWellKnown(r.URL.Path) {
				handler.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized: Authorization header missing", http.StatusUnauthorized)
				return
			}

			scheme, credentials, found := strings.Cut(authHeader, " ")
			if !found || scheme != "Basic" {
				http.Error(w, "Unauthorized: Invalid Authorization header", http.StatusUnauthorized)
				return
			}

			payload, err := base64.StdEncoding.DecodeString(credentials)
			if err != nil {
				http.Error(w, "Unauthorized: Invalid credentials format", http.StatusUnauthorized)
				return
			}

			username, password, found := strings.Cut(string(payload), ":")
			if !found {
				http.Error(w, "Unauthorized: Invalid credentials", http.StatusUnauthorized)
				return
			}

			if !validateCredentials(basicAuthProvider, username, password) {
				http.Error(w, "Unauthorized: Invalid username or password", http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}

func validateCredentials(provider BasicAuthProvider, username, password string) bool {
	// If ValidateFunc is provided, use it.
	if provider.ValidateFunc != nil {
		if provider.ValidateFunc(username, password) {
			return true
		}
	}

	// If ValidateFuncWithDatasources is provided, use it.
	if provider.ValidateFuncWithDatasources != nil {
		if provider.ValidateFuncWithDatasources(provider.Container, username, password) {
			return true
		}
	}

	storedPass, ok := provider.Users[username]

	return ok && storedPass == password
}
