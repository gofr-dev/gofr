package gofr

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/rbac"
)

var errNoRBACConfig = errors.New("no RBAC config file found at configs/rbac.json, configs/rbac.yaml or configs/rbac.yml")

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
//	if err := app.EnableRBAC(); err != nil {
//		app.Logger().Fatalf("%v", err)
//	}
//
//	// Or with custom config path
//	if err := app.EnableRBAC("configs/custom-rbac.json"); err != nil {
//		app.Logger().Fatalf("%v", err)
//	}
//
// Role extraction is configured in the config file:
// - Set "roleHeader" for header-based extraction (e.g., "X-User-Role")
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles", "roles[0]"); an array claim
// holds every role in it.
// - Set "permissionsClaimPath" (e.g., "scope") to authorize from the permissions in the token
// itself. JWT-based modes need EnableOAuth to be called first; validate the token's audience,
// issuer and expiry there, with jwt.WithAudience, jwt.WithIssuer and jwt.WithExpirationRequired.
//
// It returns an error, and installs no middleware, if the config cannot be found, read, parsed or
// validated. The error is also logged, so an app that ignores it still sees why authorization is
// off; return or exit on it to stop startup instead.
func (a *App) EnableRBAC(configPath ...string) error {
	return a.authDisabled(a.enableRBAC(configPath...), "Authorization", "EnableRBAC")
}

func (a *App) enableRBAC(configPath ...string) error {
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

	config, err := a.loadRBACConfig(path)
	if err != nil {
		return err
	}

	// RBAC metrics are opt-in and registered here (like GraphQL/gRPC/cron/LLM), when the first config
	// is installed. EnableRBAC can run more than once, so guard on the first call to avoid a
	// duplicate-metric error; this runs in the single-threaded setup phase, so no synchronization is
	// needed.
	if metrics := a.Metrics(); !a.rbacEnabled && metrics != nil {
		metrics.NewCounter("rbac_role_extraction_failures", "Number of requests whose RBAC role or permissions could not be read")
	}

	a.rbacEnabled = true

	// Apply middleware using the config
	middlewareFunc := rbac.Middleware(config)
	a.UseMiddleware(middlewareFunc)

	return nil
}

// loadRBACConfig loads the config at path and logs the startup summary. The load logs through a
// counting logger, so the summary can say that errors were logged instead of reading as a clean
// success. The returned config logs through the app logger, not the counting wrapper.
func (a *App) loadRBACConfig(path string) (*rbac.Config, error) {
	logger := a.Logger()
	loadLogger := &errorCountingLogger{Logger: logger}

	config, err := rbac.LoadPermissions(path, loadLogger, a.Metrics(), otel.GetTracerProvider().Tracer("gofr-rbac"))
	if err != nil {
		return nil, fmt.Errorf("failed to load RBAC config: %w", err)
	}

	config.Logger = logger

	if loadLogger.errors > 0 {
		logger.Errorf("Loaded RBAC config with %d error(s); see the RBAC errors logged above", loadLogger.errors)
	} else {
		logger.Infof("Loaded RBAC config successfully")
	}

	return config, nil
}

// errorCountingLogger counts the error lines logged through it while the RBAC config loads.
type errorCountingLogger struct {
	datasource.Logger
	errors int
}

func (l *errorCountingLogger) Error(args ...any) {
	l.errors++
	l.Logger.Error(args...)
}

func (l *errorCountingLogger) Errorf(format string, args ...any) {
	l.errors++
	l.Logger.Errorf(format, args...)
}
