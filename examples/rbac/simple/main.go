package main

import (
	"net/http"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Enable simple RBAC with header-based role extraction
	// The role is extracted from the "X-User-Role" header
	// Note: args will be empty for header-based RBAC (container not needed)
	app.EnableRBAC("configs/rbac.json", func(req *http.Request, args ...any) (string, error) {
		return req.Header.Get("X-User-Role"), nil
	})

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.GET("/api/admin", gofr.RequireRole("admin", adminHandler))
	app.GET("/api/dashboard", gofr.RequireAnyRole([]string{"admin", "editor"}, dashboardHandler))

	app.Run()
}

func getAllUsers(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Users list"}, nil
}

func adminHandler(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Admin panel"}, nil
}

func dashboardHandler(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Dashboard"}, nil
}

