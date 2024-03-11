package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"
)

const credentialLength = 2

type BasicAuthProvider struct {
	Users        map[string]string
	ValidateFunc func(username, password string) bool
}

func BasicAuthMiddleware(basicAuthProvider BasicAuthProvider) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized: Authorization header missing", http.StatusUnauthorized)
				return
			}

			authParts := strings.Split(authHeader, " ")
			if len(authParts) != 2 || authParts[0] != "basic" {
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

			if basicAuthProvider.ValidateFunc != nil {
				if !basicAuthProvider.ValidateFunc(credentials[0], credentials[1]) {
					http.Error(w, "Unauthorized: Invalid username or password", http.StatusUnauthorized)
					return
				}
			} else {
				if storedPass, ok := basicAuthProvider.Users[credentials[0]]; !ok || storedPass != credentials[1] {
					http.Error(w, "Unauthorized: Invalid username or password", http.StatusUnauthorized)
					return
				}
			}

			handler.ServeHTTP(w, r)
		})
	}
}
