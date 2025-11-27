package main

import (
	"gofr.dev/pkg/gofr"
	_ "gofr.dev/pkg/gofr/rbac" // Import RBAC module for automatic registration
)

func main() {
	app := gofr.New()

	// Enable OAuth middleware first (required for JWT validation)
	// Replace with your actual JWKS endpoint
	app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

	// Enable RBAC with JWT role extraction
	// The "role" parameter (roleClaim) specifies the JWT claim path
	// 
	// Supported formats:
	//   - "role" - Simple claim: {"role": "admin"}
	//   - "roles[0]" - Array notation: {"roles": ["admin", "user"]} - extracts first element
	//   - "roles[1]" - Array notation: {"roles": ["admin", "user"]} - extracts second element
	//   - "permissions.role" - Dot notation: {"permissions": {"role": "admin"}}
	//   - "user.permissions.role" - Deeply nested: {"user": {"permissions": {"role": "admin"}}}
	//
	// If roleClaim is empty (""), it defaults to "role"
	app.EnableRBAC(
		gofr.WithPermissionsFile("configs/rbac.json"),
		gofr.WithJWT("role"),
	)

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

