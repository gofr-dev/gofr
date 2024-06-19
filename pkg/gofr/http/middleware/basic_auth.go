package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"gofr.dev/pkg/gofr/container"
)

const credentialLength = 2

// BasicAuthProvider represents a basic authentication provider.
type BasicAuthProvider struct {
	Users        map[string]string
	ValidateFunc func(c *container.Container, username, password string) bool
	Container    *container.Container
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

			authParts := strings.Split(authHeader, " ")
			if len(authParts) != 2 || authParts[0] != "Basic" {
				http.Error(w, "Unauthorized: Invalid Authorization header", http.StatusUnauthorized)
				return
			}

			payload, err := base64.StdEncoding.DecodeString(authParts[1])
			if err != nil {
				http.Error(w, "Unauthorized: Invalid credentials format", http.StatusUnauthorized)
				return
			}

			credentials := strings.Split(string(payload), ":")
			if len(credentials) != credentialLength {
				http.Error(w, "Unauthorized: Invalid credentials", http.StatusUnauthorized)
				return
			}

			if !validateCredentials(basicAuthProvider, credentials) {
				http.Error(w, "Unauthorized: Invalid username or password", http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}

func validateCredentials(provider BasicAuthProvider, credentials []string) bool {
	if provider.ValidateFunc != nil {
		if !provider.ValidateFunc(provider.Container, credentials[0], credentials[1]) {
			return false
		}
	} else {
		if storedPass, ok := provider.Users[credentials[0]]; !ok || storedPass != credentials[1] {
			return false
		}
	}

	return true
}
