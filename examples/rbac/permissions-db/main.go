package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
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

	// Set RequiresContainer = true to enable container access for database queries
	config.RequiresContainer = true

	// The container is automatically passed as the first argument when RequiresContainer = true
	config.RoleExtractorFunc = func(req *http.Request, args ...any) (string, error) {
		// Extract user ID from header (could be from JWT, session, etc.)
		userID := req.Header.Get("X-User-ID")
		if userID == "" {
			return "", fmt.Errorf("user ID not found in request")
		}

		// Get container from args (automatically injected when RequiresContainer = true)
		// Container is only provided when RequiresContainer = true (database-based role extraction)
		// Access datasources through container: container.SQL, container.Redis, etc.
		if len(args) > 0 {
			if cntr, ok := args[0].(*container.Container); ok && cntr != nil && cntr.SQL != nil {
				// Use actual database if available
				var role string
				err := cntr.SQL.QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
				if err != nil {
					if err == sql.ErrNoRows {
						return "", fmt.Errorf("user not found")
					}
					return "", err
				}
				return role, nil
			}
		}

		// Fallback to mock database for demonstration when real database is not available
		userRoles := map[string]string{
			"1": "admin",
			"2": "editor",
			"3": "viewer",
		}

		role, ok := userRoles[userID]
		if !ok {
			return "", fmt.Errorf("user with ID %s not found", userID)
		}

		return role, nil
	}

	// Enable RBAC with permissions and database-based role extraction
	app.EnableRBAC(
		gofr.WithConfig(config),
		gofr.WithRoleExtractor(config.RoleExtractorFunc),
		gofr.WithPermissions(config.PermissionConfig),
		gofr.WithRequiresContainer(true),
	)

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
