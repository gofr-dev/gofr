package middleware

import (
	"context"
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

const Username authMethod = 1

// BasicAuthMiddleware creates a middleware function that enforces basic authentication using the provided BasicAuthProvider.
func BasicAuthMiddleware(basicAuthProvider BasicAuthProvider) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWellKnown(r.URL.Path) {
				handler.ServeHTTP(w, r)
				return
			}

			username, password, ok := parseBasicAuth(r)
			if !ok {
				http.Error(w, "Unauthorized: Invalid or missing Authorization header", http.StatusUnauthorized)
				return
			}

			if !validateCredentials(basicAuthProvider, username, password) {
				http.Error(w, "Unauthorized: Invalid username or password", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), Username, username)
			handler.ServeHTTP(w, r.Clone(ctx))
		})
	}
}

// parseBasicAuth extracts and decodes the username and password from the Authorization header.
func parseBasicAuth(r *http.Request) (username, password string, ok bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", false
	}

	scheme, credentials, found := strings.Cut(authHeader, " ")
	if !found || scheme != "Basic" {
		return "", "", false
	}

	payload, err := base64.StdEncoding.DecodeString(credentials)
	if err != nil {
		return "", "", false
	}

	username, password, found = strings.Cut(string(payload), ":")
	if !found { // Ensure both username and password are returned as empty if colon separator is missing
		return "", "", false
	}

	return username, password, true
}

// validateCredentials checks the provided username and password against the BasicAuthProvider.
func validateCredentials(provider BasicAuthProvider, username, password string) bool {
	if provider.ValidateFunc != nil && provider.ValidateFunc(username, password) {
		return true
	}

	if provider.ValidateFuncWithDatasources != nil && provider.ValidateFuncWithDatasources(provider.Container, username, password) {
		return true
	}

	storedPass, ok := provider.Users[username]

	return ok && storedPass == password
}
