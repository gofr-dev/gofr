package gofr

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/rbac"
)

// EnableBasicAuth enables basic authentication for the application.
//
// It takes a variable number of credentials as alternating username and password strings.
// An error is logged if an odd number of arguments is provided.
func (a *App) EnableBasicAuth(credentials ...string) {
	if len(credentials) == 0 {
		a.container.Error("No credentials provided for EnableBasicAuth. Proceeding without Authentication")
		return
	}

	if len(credentials)%2 != 0 {
		a.container.Error("Invalid number of arguments for EnableBasicAuth. Proceeding without Authentication")

		return
	}

	users := make(map[string]string)
	for i := 0; i < len(credentials); i += 2 {
		users[credentials[i]] = credentials[i+1]
	}

	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{Users: users}))
}

// EnableBasicAuthWithFunc enables basic authentication for the HTTP server with a custom validation function.
//
// Deprecated: This method is deprecated and will be removed in future releases, users must use
// [App.EnableBasicAuthWithValidator] as it has access to application datasources.
func (a *App) EnableBasicAuthWithFunc(validateFunc func(username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{ValidateFunc: validateFunc, Container: a.container}))
}

// EnableBasicAuthWithValidator enables basic authentication for the HTTP server with a custom validator.
//
// The provided `validateFunc` is invoked for each authentication attempt. It receives a container instance,
// username, and password. The function should return `true` if the credentials are valid, `false` otherwise.
func (a *App) EnableBasicAuthWithValidator(validateFunc func(c *container.Container, username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{
		ValidateFuncWithDatasources: validateFunc, Container: a.container}))
}

// EnableAPIKeyAuth enables API key authentication for the application.
//
// It requires at least one API key to be provided. The provided API keys will be used to authenticate requests.
func (a *App) EnableAPIKeyAuth(apiKeys ...string) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{}, apiKeys...))
}

// EnableAPIKeyAuthWithFunc enables API key authentication for the application with a custom validation function.
//
// Deprecated: This method is deprecated and will be removed in future releases, users must use
// [App.EnableAPIKeyAuthWithValidator] as it has access to application datasources.
func (a *App) EnableAPIKeyAuthWithFunc(validateFunc func(apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFunc: validateFunc,
		Container:    a.container,
	}))
}

// EnableAPIKeyAuthWithValidator enables API key authentication for the application with a custom validation function.
//
// The provided `validateFunc` is used to determine the validity of an API key. It receives the request container
// and the API key as arguments and should return `true` if the key is valid, `false` otherwise.
func (a *App) EnableAPIKeyAuthWithValidator(validateFunc func(c *container.Container, apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFuncWithDatasources: validateFunc,
		Container:                   a.container,
	}))
}

// EnableOAuth configures OAuth middleware for the application.
//
// It registers a new HTTP service for fetching JWKS and sets up OAuth middleware
// with the given JWKS endpoint and refresh interval.
//
// The JWKS endpoint is used to retrieve JSON Web Key Sets for verifying tokens.
// The refresh interval specifies how often to refresh the token cache.
// We can define optional JWT claim validation settings, including issuer, audience, and expiration checks.
// Accepts jwt.ParserOption for additional parsing options:
// https://pkg.go.dev/github.com/golang-jwt/jwt/v4#ParserOption
func (a *App) EnableOAuth(jwksEndpoint string,
	refreshInterval int,
	options ...jwt.ParserOption,
) {
	a.AddHTTPService("gofr_oauth", jwksEndpoint)

	oauthOption := middleware.OauthConfigs{
		Provider:        a.container.GetHTTPService("gofr_oauth"),
		RefreshInterval: time.Second * time.Duration(refreshInterval),
	}

	a.httpServer.router.Use(middleware.OAuth(middleware.NewOAuth(oauthOption), options...))
}

// EnableRBAC enables Role-Based Access Control (RBAC) for the application.
//
// It loads RBAC configuration from a JSON file and applies authorization middleware.
// The roleExtractor function is responsible for extracting the user's role from the HTTP request.
//
// Example:
//
//	app.EnableRBAC("configs/rbac.json", func(req *http.Request, args ...any) (string, error) {
//	    return req.Header.Get("X-User-Role"), nil
//	})
func (a *App) EnableRBAC(permissionsFile string, roleExtractor rbac.RoleExtractor) {
	config, err := rbac.LoadPermissions(permissionsFile)
	if err != nil {
		a.container.Errorf("Failed to load RBAC permissions: %v. Proceeding without RBAC", err)
		return
	}

	config.RoleExtractorFunc = roleExtractor
	config.Logger = a.container.Logger // Set GoFr logger for audit logging (always enabled)

	a.httpServer.router.Use(rbac.Middleware(config))
}

// EnableRBACWithConfig enables RBAC with full configuration options.
//
// This method provides maximum flexibility for RBAC configuration.
// Use this when you need custom error handling, audit logging, or other advanced features.
func (a *App) EnableRBACWithConfig(config *rbac.Config) {
	if config == nil {
		a.container.Error("RBAC config is nil. Proceeding without RBAC")
		return
	}

	// Set logger if not already set (audit logging is always enabled when logger is available)
	if config.Logger == nil {
		config.Logger = a.container.Logger
	}

	// Initialize empty maps if not present
	if config.RouteWithPermissions == nil {
		config.RouteWithPermissions = make(map[string][]string)
	}

	if config.OverRides == nil {
		config.OverRides = make(map[string]bool)
	}

	a.httpServer.router.Use(rbac.Middleware(config))
}

// EnableRBACWithHotReload enables RBAC with hot-reload capability.
//
// The configuration file will be automatically reloaded when it changes.
// The reloadInterval specifies how often to check for file changes.
// Set reloadInterval to 0 to disable hot-reload.
//
// Example:
//
//	// Reload every 30 seconds
//	app.EnableRBACWithHotReload("configs/rbac.json", roleExtractor, 30*time.Second)
func (a *App) EnableRBACWithHotReload(permissionsFile string, roleExtractor rbac.RoleExtractor, reloadInterval time.Duration) {
	loader, err := rbac.NewConfigLoaderWithLogger(permissionsFile, reloadInterval, a.container.Logger)
	if err != nil {
		a.container.Errorf("Failed to load RBAC permissions: %v. Proceeding without RBAC", err)
		return
	}

	config := loader.GetConfig()
	config.RoleExtractorFunc = roleExtractor
	config.Logger = a.container.Logger // Set GoFr logger for audit logging (always enabled)

	a.httpServer.router.Use(rbac.Middleware(config))
}

// RequireRole wraps a handler to require a specific role.
// This is a convenience wrapper that works with GoFr's Handler type.
func RequireRole(allowedRole string, handlerFunc Handler) Handler {
	rbacHandler := rbac.RequireRole(allowedRole, func(ctx interface{}) (any, error) {
		if gofrCtx, ok := ctx.(*Context); ok {
			return handlerFunc(gofrCtx)
		}
		return nil, rbac.ErrAccessDenied
	})

	return func(ctx *Context) (any, error) {
		return rbacHandler(ctx)
	}
}

// RequireAnyRole wraps a handler to require any of the specified roles.
// This is a convenience wrapper that works with GoFr's Handler type.
func RequireAnyRole(allowedRoles []string, handlerFunc Handler) Handler {
	rbacHandler := rbac.RequireAnyRole(allowedRoles, func(ctx interface{}) (any, error) {
		if gofrCtx, ok := ctx.(*Context); ok {
			return handlerFunc(gofrCtx)
		}
		return nil, rbac.ErrAccessDenied
	})

	return func(ctx *Context) (any, error) {
		return rbacHandler(ctx)
	}
}

// RequirePermission wraps a handler to require a specific permission.
// This works with permission-based access control.
// The permissionConfig must be set in the RBAC config.
func RequirePermission(requiredPermission string, permissionConfig *rbac.PermissionConfig, handlerFunc Handler) Handler {
	rbacHandler := rbac.RequirePermission(requiredPermission, permissionConfig, func(ctx interface{}) (any, error) {
		if gofrCtx, ok := ctx.(*Context); ok {
			return handlerFunc(gofrCtx)
		}
		return nil, rbac.ErrPermissionDenied
	})

	return func(ctx *Context) (any, error) {
		return rbacHandler(ctx)
	}
}

// EnableRBACWithPermissions enables RBAC with permission-based access control.
//
// This method supports both role-based and permission-based authorization.
// Permissions provide finer-grained control than roles alone.
//
// Example:
//
//	config, _ := rbac.LoadPermissions("configs/rbac.json")
//	config.PermissionConfig = &rbac.PermissionConfig{
//	    Permissions: map[string][]string{
//	        "users:read": ["admin", "editor", "viewer"],
//	        "users:write": ["admin", "editor"],
//	    },
//	    RoutePermissionMap: map[string]string{
//	        "GET /api/users": "users:read",
//	        "POST /api/users": "users:write",
//	    },
//	}
//	app.EnableRBACWithPermissions(config, roleExtractor)
func (a *App) EnableRBACWithPermissions(config *rbac.Config, roleExtractor rbac.RoleExtractor) {
	if config == nil {
		a.container.Error("RBAC config is nil. Proceeding without RBAC")
		return
	}

	config.RoleExtractorFunc = roleExtractor
	config.EnablePermissions = true
	if config.Logger == nil {
		config.Logger = a.container.Logger // Set GoFr logger for audit logging (always enabled)
	}

	// Initialize empty maps if not present
	if config.RouteWithPermissions == nil {
		config.RouteWithPermissions = make(map[string][]string)
	}

	if config.OverRides == nil {
		config.OverRides = make(map[string]bool)
	}

	a.httpServer.router.Use(rbac.Middleware(config))
}

// EnableRBACWithJWT enables RBAC with JWT-based role extraction.
//
// This method integrates with GoFr's OAuth middleware to extract roles from JWT claims.
// The OAuth middleware must be enabled before calling this method.
//
// The roleClaim parameter specifies the path to the role in JWT claims:
//   - "role" for simple claim: {"role": "admin"}
//   - "roles[0]" for array: {"roles": ["admin", "user"]}
//   - "permissions.role" for nested: {"permissions": {"role": "admin"}}
//
// Example:
//
//	app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)
//	app.EnableRBACWithJWT("configs/rbac.json", "role")
func (a *App) EnableRBACWithJWT(permissionsFile string, roleClaim string) {
	config, err := rbac.LoadPermissions(permissionsFile)
	if err != nil {
		a.container.Errorf("Failed to load RBAC permissions: %v. Proceeding without RBAC", err)
		return
	}

	// Create JWT role extractor
	jwtExtractor := rbac.NewJWTRoleExtractor(roleClaim)
	config.RoleExtractorFunc = jwtExtractor.ExtractRole
	config.Logger = a.container.Logger // Set GoFr logger for audit logging (always enabled)

	a.httpServer.router.Use(rbac.Middleware(config))
}
