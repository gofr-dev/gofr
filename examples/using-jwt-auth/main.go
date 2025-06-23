package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/examples/using-jwt-auth/handler"
	// "gofr.dev/examples/using-jwt-auth/middleware" // 🔒 Middleware disabled for now
)

func main() {
	app := gofr.New()

	secret := "your-256-bit-secret" // Replace with env or secure value in production

	// ✅ Public route to get JWT
	app.GET("/login", handler.LoginHandler(secret))

	// 🚫 Secure route using JWT middleware (commented until header access is supported)
	/*
		app.GET("/secure", middleware.JWTAuth(secret)(func(ctx *gofr.Context) (interface{}, error) {
			user := ctx.Context.Value("user")
			return map[string]interface{}{
				"message": "Secure route accessed",
				"user":    user,
			}, nil
		}))
	*/

	// ✅ App run
	app.Run()
}
// Note: The JWT middleware is commented out because the current version of gofr does not support header access in middleware.