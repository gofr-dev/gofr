package gofr

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"gofr.dev/pkg/gofr/container"
)

var (
	errRBACModuleNotImportedAccess     = errors.New("forbidden: access denied - RBAC module not imported")
	errRBACModuleNotImportedPermission = errors.New("forbidden: permission denied - RBAC module not imported")
	errRoleHeaderNotFound              = errors.New("role header not found")
)

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
// Example:
//
//	// Use default path (configs/rbac.json or configs/rbac.yaml)
//	app.EnableRBAC()
//
//	// Use custom path
//	app.EnableRBAC("configs/custom-rbac.json")
//
//	// Use default path with JWT option
//	app.EnableRBAC("", &rbac.JWTExtractor{Claim: "role"})
//
//	// Use custom path with options
//	app.EnableRBAC("configs/rbac.json", &rbac.HeaderRoleExtractor{HeaderKey: "X-User-Role"})
//
// Note: When using RBAC options (e.g., &rbac.JWTExtractor{}), you must import the rbac package.
// The rbac package's init() function will automatically register itself when imported.
// Example: import "gofr.dev/pkg/gofr/rbac"
//
// Options follow the same pattern as service.Options - each option implements AddOption method.
// The provider parameter follows the same pattern as datasources (e.g., app.AddMongo(mongoProvider)).
// Users create the provider from the rbac package: provider := rbac.NewProvider()
// Options can be either gofr.RBACOption or rbac.Options (both are accepted).
func (a *App) EnableRBAC(provider container.RBACProvider, configFile string, options ...any) {
	if provider == nil {
		a.container.Error("RBAC provider is required. Create one using: provider := rbac.NewProvider()")
		return
	}

	// Resolve config file path
	filePath := resolveRBACConfigPath(configFile)
	if filePath == "" {
		a.container.Warn("RBAC config file not found. Tried: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml")
		return
	}

	// Load configuration from file using the provider
	configAny, err := provider.LoadPermissions(filePath)
	if err != nil {
		a.container.Errorf("Failed to load RBAC config from %s: %v", filePath, err)
		return
	}

	// Type assert to RBACConfig
	// The provider returns *rbac.Config which implements both rbac.RBACConfig and gofr.RBACConfig
	var config RBACConfig
	if gofrConfig, ok := configAny.(RBACConfig); ok {
		config = gofrConfig
	} else {
		a.container.Error("RBAC provider returned invalid config type")
		return
	}

	a.container.Infof("Loaded RBAC config from %s", filePath)

	// Auto-configure header extractor if RoleHeader is in config
	autoConfigureHeaderExtractor(config)

	// Apply user-provided options (following service.Options pattern)
	// Options can be either gofr.RBACOption or rbac.Options
	// Since *rbac.Config implements both rbac.RBACConfig and gofr.RBACConfig
	// (they have identical method signatures), we can pass the config directly
	for _, opt := range options {
		// Try gofr.RBACOption first
		if gofrOpt, ok := opt.(RBACOption); ok {
			config = gofrOpt.AddOption(config)
		} else if rbacOpt, ok := opt.(interface {
			AddOption(interface{}) interface{}
		}); ok {
			// Handle rbac.Options (from rbac package)
			// Since *rbac.Config implements both interfaces, we can pass it
			result := rbacOpt.AddOption(config)
			if resultConfig, ok := result.(RBACConfig); ok {
				config = resultConfig
			}
		}
	}

	// Setup logger
	if config.GetLogger() == nil {
		config.SetLogger(a.container.Logger)
	}

	// Initialize maps
	config.InitializeMaps()

	// Auto-detect permissions if PermissionConfig is set
	if config.GetPermissionConfig() != nil {
		config.SetEnablePermissions(true)
	}

	// Apply middleware using the provider
	middlewareFunc := provider.GetMiddleware(config)
	a.httpServer.router.Use(middlewareFunc)

	// Store provider for RequireRole, RequireAnyRole, RequirePermission functions
	a.container.RBAC = provider
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

// autoConfigureHeaderExtractor automatically configures header-based role extraction
// if RoleHeader is set in the config file.
func autoConfigureHeaderExtractor(config RBACConfig) {
	if roleHeader := config.GetRoleHeader(); roleHeader != "" {
		// Create header extractor and apply it
		config.SetRoleExtractorFunc(func(req *http.Request, _ ...any) (string, error) {
			role := req.Header.Get(roleHeader)
			if role == "" {
				return "", fmt.Errorf("%w: %q", errRoleHeaderNotFound, roleHeader)
			}

			return role, nil
		})
	}
}


// RequireRole wraps a handler to require a specific role.
// This is a convenience wrapper that works with GoFr's Handler type.
//
// Note: RBAC must be enabled via app.EnableRBAC() before using this function.
func RequireRole(allowedRole string, handlerFunc Handler) Handler {
	// Get RBAC provider from context (set by EnableRBAC)
	// This follows the same pattern as accessing datasources from context
	return func(ctx *Context) (any, error) {
		provider := ctx.RBAC
		if provider == nil {
			return nil, errRBACModuleNotImportedAccess
		}

		rbacHandler := provider.RequireRole(allowedRole, func(ctx any) (any, error) {
			if gofrCtx, ok := ctx.(*Context); ok {
				return handlerFunc(gofrCtx)
			}

			return nil, provider.ErrAccessDenied()
		})

		return rbacHandler(ctx)
	}
}

// RequireAnyRole wraps a handler to require any of the specified roles.
// This is a convenience wrapper that works with GoFr's Handler type.
//
// Note: RBAC must be enabled via app.EnableRBAC() before using this function.
func RequireAnyRole(allowedRoles []string, handlerFunc Handler) Handler {
	return func(ctx *Context) (any, error) {
		provider := ctx.RBAC
		if provider == nil {
			return nil, errRBACModuleNotImportedAccess
		}

		rbacHandler := provider.RequireAnyRole(allowedRoles, func(ctx any) (any, error) {
			if gofrCtx, ok := ctx.(*Context); ok {
				return handlerFunc(gofrCtx)
			}

			return nil, provider.ErrAccessDenied()
		})

		return rbacHandler(ctx)
	}
}

// RequirePermission wraps a handler to require a specific permission.
// This works with permission-based access control.
// The permissionConfig must be set in the RBAC config.
//
// Note: RBAC must be enabled via app.EnableRBAC() before using this function.
func RequirePermission(requiredPermission string, permissionConfig PermissionConfig, handlerFunc Handler) Handler {
	return func(ctx *Context) (any, error) {
		provider := ctx.RBAC
		if provider == nil {
			return nil, errRBACModuleNotImportedPermission
		}

		rbacHandler := provider.RequirePermission(requiredPermission, permissionConfig, func(ctx any) (any, error) {
			if gofrCtx, ok := ctx.(*Context); ok {
				return handlerFunc(gofrCtx)
			}

			return nil, provider.ErrPermissionDenied()
		})

		return rbacHandler(ctx)
	}
}
