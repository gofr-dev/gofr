package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Enable OAuth middleware first (required for JWT validation)
	// Replace with your actual JWKS endpoint
	app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

	// Enable RBAC with JWT role extraction
	// The "role" parameter specifies the JWT claim path
	// Supports: "role", "roles[0]", "permissions.role", etc.
	app.EnableRBACWithJWT("configs/rbac.json", "role")

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.GET("/api/admin", adminHandler)

	app.Run()
}

func getAllUsers(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Users list"}, nil
}

func adminHandler(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Admin panel"}, nil
}

