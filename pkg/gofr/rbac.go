package gofr

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/rbac"
)

var errNoRBACConfig = errors.New("no RBAC config file found at configs/rbac.json, configs/rbac.yaml or configs/rbac.yml")

// EnableRBACWithError enables RBAC by loading configuration from a JSON or YAML file.
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
//	if err := app.EnableRBACWithError(); err != nil {
//		app.Logger().Fatalf("%v", err)
//	}
//
//	// Or with custom config path
//	err := app.EnableRBACWithError("configs/custom-rbac.json")
//
// Role extraction is configured in the config file:
// - Set "roleHeader" for header-based extraction (e.g., "X-User-Role")
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles[0]").
//
// It returns an error, and installs no middleware, if the config cannot be found, read, parsed or
// validated.
func (a *App) EnableRBACWithError(configPath ...string) error {
	var path string
	if len(configPath) > 0 {
		path = configPath[0]
	} else {
		// Use rbac.DefaultConfigPath (empty string) to trigger default path resolution
		path = rbac.ResolveRBACConfigPath(rbac.DefaultConfigPath)
		if path == "" {
			return errNoRBACConfig
		}
	}

	// Get dependencies
	logger := a.Logger()
	metrics := a.Metrics()
	tracer := otel.GetTracerProvider().Tracer("gofr-rbac")

	// Load configuration directly with dependencies
	config, err := rbac.LoadPermissions(path, logger, metrics, tracer)
	if err != nil {
		return fmt.Errorf("failed to load RBAC config: %w", err)
	}

	a.Logger().Infof("Loaded RBAC config successfully")

	// Apply middleware using the config
	middlewareFunc := rbac.Middleware(config)
	a.UseMiddleware(middlewareFunc)

	a.rbacConfig = config
	a.rbacStrict = true

	return nil
}

// EnableRBAC enables RBAC by loading configuration from a JSON or YAML file.
//
// Deprecated: use [App.EnableRBACWithError], which returns the error instead of starting with
// authorization disabled. EnableRBAC will be removed in the next major release.
func (a *App) EnableRBAC(configPath ...string) {
	if err := a.EnableRBACWithError(configPath...); err != nil {
		a.Logger().Errorf(authDisabledMsg, err, "Authorization", "EnableRBACWithError")

		return
	}

	a.rbacStrict = false
}
