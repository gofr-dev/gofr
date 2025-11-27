package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// Enable OAuth middleware first (required for JWT validation)
	app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

	// Load RBAC configuration
	config, err := rbac.LoadPermissions("configs/rbac.json")
	if err != nil {
		app.Logger().Error("Failed to load RBAC config: ", err)
		return
	}

	// Configure permission-based access control
	config.PermissionConfig = &rbac.PermissionConfig{
		Permissions: map[string][]string{
			"users:read":   {"admin", "editor", "viewer"},
			"users:write":  {"admin", "editor"},
			"users:delete": {"admin"},
			"posts:read":   {"admin", "author", "viewer"},
			"posts:write":  {"admin", "author"},
		},
		RoutePermissionMap: map[string]string{
			"GET /api/users":    "users:read",
			"POST /api/users":   "users:write",
			"DELETE /api/users": "users:delete",
			"GET /api/posts":    "posts:read",
			"POST /api/posts":   "posts:write",
		},
	}

	// Enable RBAC with permissions and JWT role extraction
	// The roleClaim parameter supports multiple formats:
	//   - "role" - Simple claim: {"role": "admin"}
	//   - "roles[0]" - Array notation: {"roles": ["admin", "user"]}
	//   - "permissions.role" - Dot notation: {"permissions": {"role": "admin"}}
	//   - "user.permissions.role" - Deeply nested: {"user": {"permissions": {"role": "admin"}}}
	// If empty (""), defaults to "role"
	app.EnableRBAC(
		gofr.WithConfig(config),
		gofr.WithJWT("role"),
		gofr.WithPermissions(config.PermissionConfig),
	)

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.POST("/api/users", createUser)
	app.DELETE("/api/users", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
	app.GET("/api/posts", getAllPosts)
	app.POST("/api/posts", createPost)

	app.Run()
}

func getAllUsers(_ *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Users list"}, nil
}

func createUser(_ *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "User created"}, nil
}

func deleteUser(_ *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "User deleted"}, nil
}

func getAllPosts(_ *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Posts list"}, nil
}

func createPost(_ *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Post created"}, nil
}

