package main

import (
	"net/http"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// Example 1: Hot-reload configuration
	// Configuration will be automatically reloaded every 30 seconds
	app.EnableRBACWithHotReload("configs/rbac.yaml", func(req *http.Request, args ...any) (string, error) {
		return req.Header.Get("X-User-Role"), nil
	}, 30*time.Second)

	// Example 2: Custom error handler
	config, _ := rbac.LoadPermissions("configs/rbac.json")
	config.ErrorHandler = func(w http.ResponseWriter, r *http.Request, role, route string, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "Access denied", "role": "` + role + `", "route": "` + route + `"}`))
	}

	// Example 4: Enable caching
	config.EnableCache = true
	config.CacheTTL = 5 * time.Minute

	app.EnableRBACWithConfig(config)

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.GET("/api/admin", gofr.RequireRole("admin", adminHandler))

	app.Run()
}

func getAllUsers(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Users list"}, nil
}

func adminHandler(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Admin panel"}, nil
}
