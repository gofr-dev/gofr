package main

import (
	"os"

	"gofr.dev/pkg/gofr"
	"gofr.dev/examples/using-jwt-auth/handler"
	// "gofr.dev/examples/using-jwt-auth/middleware" // 🔒 Middleware disabled for now
)

func main() {
	app := gofr.New()

	//  Best practice: Load secret from environment variable
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		app.Logger().Warn("JWT_SECRET environment variable not set. Using fallback secret for local testing.")
		secret = "your-256-bit-secret" // ⚠️ Replace this for production
	}

	//  Public route to get JWT
	app.GET("/login", handler.LoginHandler(secret))

	//  Secure route using JWT middleware (commented until header access is supported in GoFr)
	/*
		app.GET("/secure", middleware.JWTAuth(secret)(func(ctx *gofr.Context) (interface{}, error) {
			user := ctx.Context.Value("user")
			return map[string]interface{}{
				"message": "Secure route accessed",
				"user":    user,
			}, nil
		}))
	*/

	// ✅ Run the application
	app.Run()
}
