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

	// Create JWT role extractor
	jwtExtractor := rbac.NewJWTRoleExtractor("role")
	config.RoleExtractorFunc = jwtExtractor.ExtractRole

	// Enable RBAC with permissions
	app.EnableRBACWithPermissions(config, jwtExtractor.ExtractRole)

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.POST("/api/users", createUser)
	app.DELETE("/api/users", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
	app.GET("/api/posts", getAllPosts)
	app.POST("/api/posts", createPost)

	app.Run()
}

func getAllUsers(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Users list"}, nil
}

func createUser(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "User created"}, nil
}

func deleteUser(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "User deleted"}, nil
}

func getAllPosts(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Posts list"}, nil
}

func createPost(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Post created"}, nil
}

