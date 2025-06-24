package middleware

import (
	"gofr.dev/pkg/gofr"
)

// JWTAuth is a placeholder middleware for securing routes via JWT authentication.
// Currently non-functional due to GoFr not yet exposing header access.
// When supported, this middleware will:
//   - Read the Authorization header
//   - Extract and validate the Bearer token
//   - Attach user claims to the context for downstream access
func JWTAuth(secret string) func(gofr.Handler) gofr.Handler {
	return func(next gofr.Handler) gofr.Handler {
		return func(ctx *gofr.Context) (interface{}, error) {

			/* 
			// ❌ Not supported yet — ctx.Request.GetHeader() is unavailable

			authHeader := ctx.Request.GetHeader("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				ctx.Error(http.StatusUnauthorized, "Missing or malformed token")
				return nil, nil
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				ctx.Error(http.StatusUnauthorized, "Invalid token")
				return nil, nil
			}

			// Attach user claims to context if needed in future handlers
			ctx.Context = context.WithValue(ctx.Context, "user", token.Claims)
			*/

			// Temporary: Skip auth check until header access is supported
			return next(ctx)
		}
	}
}
