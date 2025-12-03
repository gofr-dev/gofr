package gofr

import (
	"net/http"
	"os"
)

// RBACProvider is the interface for RBAC implementations.
// External RBAC modules (like gofr.dev/pkg/gofr/rbac) implement this interface. .
// Note: This interface uses `any` for types to avoid cyclic imports with rbac package.
type RBACProvider interface {
	// LoadPermissions loads RBAC configuration from a file
	LoadPermissions(file string) (any, error)

	// GetMiddleware returns the middleware function for the given config
	// The returned function should be compatible with http.Handler middleware pattern
	GetMiddleware(config any) func(http.Handler) http.Handler

	// EnableHotReload enables hot reloading with the given source.
	// This should be called in app.OnStart after Redis/HTTP services are available.
	// The config file must have hotReload.enabled: true for this to work.
	EnableHotReload(source HotReloadSource) error
}

// HotReloadSource is the interface for RBAC hot reload sources.
// Implementations can fetch updated RBAC config from Redis, HTTP service, etc.
type HotReloadSource interface {
	// FetchConfig fetches the updated RBAC configuration
	// Returns the config data (JSON or YAML bytes) and error
	FetchConfig() ([]byte, error)
}

const (
	// Default RBAC config paths (tried in order).
	defaultRBACJSONPath = "configs/rbac.json"
	defaultRBACYAMLPath = "configs/rbac.yaml"
	defaultRBACYMLPath  = "configs/rbac.yml"
)

// EnableRBAC enables RBAC by loading configuration from a JSON or YAML file.
// This is a factory function that registers RBAC implementations and sets up the middleware.
// If configFile is empty, tries default paths: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml
//
// Pure config-based: All authorization rules are defined in the config file using:
// - Roles: role → permission mapping (format: "resource:action")
// - Endpoints: route & method → permission mapping
//
// Example:
//
//	import (
//		"gofr.dev/pkg/gofr"
//		"gofr.dev/pkg/gofr/rbac"
//	)
//
//	app := gofr.New()
//	provider := rbac.NewProvider()
//	app.EnableRBAC(provider, "configs/rbac.json") // Uses default path if empty
//
// Role extraction is configured in the config file:
// - Set "roleHeader" for header-based extraction (e.g., "X-User-Role")
// - Set "jwtClaimPath" for JWT-based extraction (e.g., "role", "roles[0]")
//
// Hot reload can be configured in the config file:
// - Set "hotReload.enabled": true
// - Set "hotReload.intervalSeconds": 60
// - Configure hot reload source programmatically (Redis/HTTP service)
func (a *App) EnableRBAC(provider RBACProvider, configFile string) {
	if provider == nil {
		a.Logger().Error("RBAC provider is required. Create one using: provider := rbac.NewProvider()")
		return
	}

	// Resolve config file path
	filePath := resolveRBACConfigPath(configFile)
	if filePath == "" {
		a.Logger().Warn("RBAC config file not found. Tried: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml")
		return
	}

	// Set logger automatically (same pattern as DBResolver)
	if rbacProvider, ok := provider.(interface{ UseLogger(any) }); ok {
		rbacProvider.UseLogger(a.Logger())
	}

	// Load configuration from file using the provider
	// Logger is automatically set on config during LoadPermissions
	config, err := provider.LoadPermissions(filePath)
	if err != nil {
		a.Logger().Errorf("Failed to load RBAC config from %s: %v", filePath, err)
		return
	}

	a.Logger().Infof("Loaded RBAC config from %s", filePath)

	// Note: Hot reload is not started here because the source (Redis/HTTP) needs to be
	// configured in app.OnStart hook after Redis/HTTP services are available.
	// Users should configure hot reload source in app.OnStart and call config.StartHotReload()

	// Apply middleware using the provider
	middlewareFunc := provider.GetMiddleware(config)
	a.httpServer.router.Use(middlewareFunc)
}

// resolveRBACConfigPath resolves the RBAC config file path.
func resolveRBACConfigPath(configFile string) string {
	// If custom path provided, use it
	if configFile != "" {
		return configFile
	}

	// Try default paths in order
	defaultPaths := []string{
		defaultRBACJSONPath,
		defaultRBACYAMLPath,
		defaultRBACYMLPath,
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
