package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"
)

const credentialLength = 2

type AuthenticationProvider interface {
	ValidateUser(username, password string) bool
}

func BasicAuthMiddleware(authProvider AuthenticationProvider) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			authParts := strings.Split(authHeader, " ")
			if len(authParts) != 2 || authParts[0] != "basic" {
				http.Error(w, "Invalid Authorization header", http.StatusUnauthorized)
				return
			}

			payload, err := base64.StdEncoding.DecodeString(authParts[1])
			if err != nil {
				http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
				return
			}

			credentials := strings.SplitN(string(payload), ":", credentialLength)
			if len(credentials) != credentialLength {
				http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
				return
			}

			if !authProvider.ValidateUser(credentials[0], credentials[1]) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
