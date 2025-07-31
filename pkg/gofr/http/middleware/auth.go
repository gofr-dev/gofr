package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// authMethod represents a custom type to define the different authentication methods supported.
type authMethod int

const (
	JWTClaim authMethod = iota // JWTClaim represents the key used to store JWT claims within the request context.
	Username
	APIKey

	// #nosec G101
	headerXAPIKey       = "X-Api-Key"
	headerAuthorization = "Authorization"
)

var (
	errContainerNil      = errors.New("container is nil")
	errValidateFuncEmpty = errors.New("validate func is empty")
)

type AuthProvider interface {
	getAuthMethod() authMethod
	extractAuthHeader(r *http.Request) (any, ErrorHTTP)
}

// AuthMiddleware creates a middleware function that enforces authentication based on the method provided.
func AuthMiddleware(a AuthProvider) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWellKnown(r.URL.Path) {
				handler.ServeHTTP(w, r)
				return
			}

			authHeader, err := a.extractAuthHeader(r)
			if err != nil {
				http.Error(w, err.Error(), err.StatusCode())
				return
			}

			ctx := context.WithValue(r.Context(), a.getAuthMethod(), authHeader)
			*r = *r.Clone(ctx)

			handler.ServeHTTP(w, r)
		})
	}
}

// getAuthHeaderFromRequest returns the auth value from header.
func getAuthHeaderFromRequest(r *http.Request, key, prefix string) (string, ErrorHTTP) {
	header := r.Header.Get(key)

	if header == "" {
		return header, NewMissingAuthHeaderError(key)
	}

	if prefix == "" {
		return header, nil
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != prefix || parts[1] == "" {
		return "", NewInvalidAuthorizationHeaderFormatError(key, fmt.Sprintf("header should be `%s <value>`", prefix))
	}

	return parts[1], nil
}
