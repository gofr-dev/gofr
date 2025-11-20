package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

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

	// Database-based role extraction
	// Extract user ID from header/token, then query database for role
	config.RoleExtractorFunc = func(req *http.Request, args ...any) (string, error) {
		// Extract user ID from header (could be from JWT, session, etc.)
		userID := req.Header.Get("X-User-ID")
		if userID == "" {
			return "", fmt.Errorf("user ID not found in request")
		}

		// Query database for user's role
		// In a real application, you would use GoFr's database connection
		var role string
		err := app.DB().QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", fmt.Errorf("user not found")
			}
			return "", err
		}

		return role, nil
	}

	// Enable RBAC with permissions
	app.EnableRBACWithPermissions(config, config.RoleExtractorFunc)

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

