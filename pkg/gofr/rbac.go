package gofr

import (
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/rbac"
)

// EnableRBAC enables RBAC by loading configuration from a JSON or YAML file.
// It loads the config directly and sets up the middleware.
//
// Pure config-based: All authorization rules are defined in the config file using:
// - Roles: role → permission mapping (format: "resource:action")
// - Endpoints: route & method → permission mapping
//
// Usage:
//
//	// Use default paths (configs/rbac.json, configs/rbac.yaml, configs/rbac.yml)
//	// Uses rbac.DefaultConfigPath internally
//	app.EnableRBAC()
//
//	// Or with custom config path
//	app.EnableRBAC("configs/custom-rbac.json")
//
// Role extraction is configured in the config file:
// - Set "roleHeader" for header-based extraction (e.g., "X-User-Role")
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles[0]").
func (a *App) EnableRBAC(configPath ...string) {
	var path string
	if len(configPath) > 0 {
		path = configPath[0]
	} else {
		// Use rbac.DefaultConfigPath (empty string) to trigger default path resolution
		path = rbac.ResolveRBACConfigPath(rbac.DefaultConfigPath)
	}

	// Get dependencies
	logger := a.Logger()
	metrics := a.Metrics()
	tracer := otel.GetTracerProvider().Tracer("gofr-rbac")

	// Load configuration directly with dependencies
	config, err := rbac.LoadPermissions(path, logger, metrics, tracer)
	if err != nil {
		a.Logger().Errorf("Failed to load RBAC config: %v", err)
		return
	}

	a.Logger().Infof("Loaded RBAC config successfully")

	// Apply middleware using the config
	middlewareFunc := rbac.Middleware(config)
	a.UseMiddleware(middlewareFunc)
}
