// File: pkg/gofr/http/middleware/oidc.go

package middleware

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"
    "time"
)

// ctxKey is the type used for the context value key to avoid collisions.
type ctxKey int

const (
    userInfoKey ctxKey = iota
)

// OIDCUserInfoMiddleware returns a middleware that fetches user info from the OIDC userinfo endpoint.
// It expects a valid Bearer token is present in the Authorization header (already validated by GoFr's OAuth middleware).
// Put this middleware **after** GoFr's OAuth middleware.
func OIDCUserInfoMiddleware(userInfoEndpoint string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if !strings.HasPrefix(authHeader, "Bearer ") {
                http.Error(w, "Unauthorized: missing bearer token", http.StatusUnauthorized)
                return
            }
            accessToken := strings.TrimPrefix(authHeader, "Bearer ")

            req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, userInfoEndpoint, nil)
            if err != nil {
                http.Error(w, "Failed to create userinfo request", http.StatusInternalServerError)
                return
            }
            req.Header.Set("Authorization", "Bearer "+accessToken)

            client := &http.Client{Timeout: 5 * time.Second}
            resp, err := client.Do(req)
            if err != nil {
                http.Error(w, "Failed to fetch userinfo", http.StatusUnauthorized)
                return
            }
            defer resp.Body.Close()

            if resp.StatusCode != http.StatusOK {
                http.Error(w, "Userinfo endpoint returned error", http.StatusUnauthorized)
                return
            }

            var userInfo map[string]interface{}
            if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
                http.Error(w, "Failed to parse userinfo response", http.StatusInternalServerError)
                return
            }

            // Store userInfo in the request context for access in GoFr handlers
            ctx := context.WithValue(r.Context(), userInfoKey, userInfo)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// GetOIDCUserInfo extracts user info from the context inside GoFr handlers.
func GetOIDCUserInfo(ctx context.Context) (map[string]interface{}, bool) {
    userInfo, ok := ctx.Value(userInfoKey).(map[string]interface{})
    return userInfo, ok
}

