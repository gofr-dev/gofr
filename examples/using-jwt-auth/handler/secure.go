package handler

import (
	"gofr.dev/pkg/gofr"
)

// SecureHandler is a protected route that should only be accessible with a valid JWT.
// This will only work once GoFr supports header access in middleware.
func SecureHandler(ctx *gofr.Context) (interface{}, error) {
	// When header access is supported, the middleware will inject user info into context.
	user := ctx.Context.Value("user")

	return map[string]interface{}{
		"message": "✅ Secure route accessed",
		"user":    user,
	}, nil
}
