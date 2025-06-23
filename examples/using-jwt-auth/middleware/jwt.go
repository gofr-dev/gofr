package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/pkg/gofr"
)

// JWTAuth is a placeholder for JWT middleware.  This won't work until GoFr exposes header access.
// Once supported, this middleware will extract a Bearer token from the Authorization header,
// validate it, and attach the claims to the context.
func JWTAuth(secret string) func(gofr.Handler) gofr.Handler {
	return func(next gofr.Handler) gofr.Handler {
		return func(ctx *gofr.Context) (interface{}, error) {

			// ❌ Currently not supported — ctx.Request.GetHeader() doesn't exist
			/*
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

				ctx.Context = context.WithValue(ctx.Context, "user", token.Claims)
			*/

			// Middleware disabled — just call next handler directly
			return next(ctx)
		}
	}
}
