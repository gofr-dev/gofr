package gofr

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
)

var (
	errRBACModuleNotImportedAccess     = errors.New("forbidden: access denied - RBAC module not imported")
	errRBACModuleNotImportedPermission = errors.New("forbidden: permission denied - RBAC module not imported")
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

// RBACOptions holds configuration options for RBAC.
type RBACOptions struct {
	// PermissionsFile is the path to the RBAC configuration file (JSON or YAML)
	PermissionsFile string

	// RoleExtractor extracts the user's role from the HTTP request
	RoleExtractor RoleExtractor

	// Config is a pre-loaded RBAC configuration
	// If provided, PermissionsFile is ignored
	Config RBACConfig

	// JWTRoleClaim specifies the JWT claim path for role extraction
	// Examples: "role", "roles[0]", "permissions.role"
	// If set, RoleExtractor is ignored
	JWTRoleClaim string

	// PermissionConfig enables permission-based access control
	PermissionConfig PermissionConfig

	// RequiresContainer indicates if container access is needed for role extraction
	RequiresContainer bool
}

// RBACOption is a function that configures RBACOptions.
type RBACOption func(*RBACOptions)

// WithPermissionsFile sets the RBAC configuration file path.
func WithPermissionsFile(file string) RBACOption {
	return func(o *RBACOptions) {
		o.PermissionsFile = file
	}
}

// WithRoleExtractor sets the role extractor function.
func WithRoleExtractor(extractor RoleExtractor) RBACOption {
	return func(o *RBACOptions) {
		o.RoleExtractor = extractor
	}
}

// WithConfig sets a pre-loaded RBAC configuration.
func WithConfig(config RBACConfig) RBACOption {
	return func(o *RBACOptions) {
		o.Config = config
	}
}

// WithJWT sets JWT-based role extraction using the specified claim path.
func WithJWT(roleClaim string) RBACOption {
	return func(o *RBACOptions) {
		o.JWTRoleClaim = roleClaim
	}
}

// WithPermissions enables permission-based access control.
func WithPermissions(permissionConfig PermissionConfig) RBACOption {
	return func(o *RBACOptions) {
		o.PermissionConfig = permissionConfig
		o.RequiresContainer = false // Permissions don't need container by default
	}
}

// WithRequiresContainer sets whether container access is needed for role extraction.
func WithRequiresContainer(required bool) RBACOption {
	return func(o *RBACOptions) {
		o.RequiresContainer = required
	}
}

// EnableRBAC enables Role-Based Access Control (RBAC) for the application.
//
// It supports various configuration options through functional options:
//   - WithPermissionsFile: Load RBAC config from a file
//   - WithRoleExtractor: Set custom role extraction function
//   - WithConfig: Use a pre-loaded RBAC configuration
//   - WithJWT: Enable JWT-based role extraction
//   - WithPermissions: Enable permission-based access control
//   - WithRequiresContainer: Indicate if container access is needed
//
// Note: This requires importing gofr.dev/pkg/gofr/rbac module.
//
// Examples:
//
//	// Simple RBAC with header-based role extraction
//	app.EnableRBAC(
//	    WithPermissionsFile("configs/rbac.json"),
//	    WithRoleExtractor(func(req *http.Request, args ...any) (string, error) {
//	        return req.Header.Get("X-User-Role"), nil
//	    }),
//	)
//
//	// RBAC with JWT role extraction
//	app.EnableRBAC(
//	    WithPermissionsFile("configs/rbac.json"),
//	    WithJWT("role"),
//	)
//
//	// RBAC with permissions
//	app.EnableRBAC(
//	    WithConfig(config),
//	    WithRoleExtractor(roleExtractor),
//	    WithPermissions(permissionConfig),
//	)
func (a *App) EnableRBAC(options ...RBACOption) {
	if rbacRegistry.middleware == nil {
		a.container.Error("RBAC module not imported. Import gofr.dev/pkg/gofr/rbac to use RBAC features")
		return
	}

	opts := a.applyRBACOptions(options)

	config := a.loadRBACConfig(opts)
	if config == nil {
		return
	}

	a.configureRBAC(config, opts)
	a.applyRBACMiddleware(config)
}

func (*App) applyRBACOptions(options []RBACOption) *RBACOptions {
	opts := &RBACOptions{}
	for _, opt := range options {
		opt(opts)
	}

	return opts
}

func (a *App) loadRBACConfig(opts *RBACOptions) RBACConfig {
	if opts.Config != nil {
		return opts.Config
	}

	if opts.PermissionsFile == "" {
		a.container.Error("RBAC configuration not provided. Use WithPermissionsFile or WithConfig option")
		return nil
	}

	if rbacRegistry.loader == nil {
		a.container.Error("RBAC module not imported. Import gofr.dev/pkg/gofr/rbac to use RBAC features")
		return nil
	}

	return a.loadRBACConfigFromFile(opts)
}

func (a *App) loadRBACConfigFromFile(opts *RBACOptions) RBACConfig {
	config, err := rbacRegistry.loader.LoadPermissions(opts.PermissionsFile)
	if err != nil {
		a.container.Errorf("Failed to load RBAC permissions: %v. Proceeding without RBAC", err)
		return nil
	}

	return config
}

func (a *App) configureRBAC(config RBACConfig, opts *RBACOptions) {
	a.configureRoleExtractor(config, opts)

	if opts.PermissionConfig != nil {
		config.SetEnablePermissions(true)
	}

	if opts.RequiresContainer {
		config.SetRequiresContainer(true)
	}

	if config.GetLogger() == nil {
		config.SetLogger(a.container.Logger)
	}

	config.InitializeMaps()
}

func (a *App) configureRoleExtractor(config RBACConfig, opts *RBACOptions) {
	if opts.JWTRoleClaim != "" {
		a.configureJWTExtractor(config, opts.JWTRoleClaim)
	} else if opts.RoleExtractor != nil {
		config.SetRoleExtractorFunc(opts.RoleExtractor)
	}
}

func (a *App) configureJWTExtractor(config RBACConfig, roleClaim string) {
	if rbacRegistry.loader == nil {
		a.container.Error("RBAC module not imported. Import gofr.dev/pkg/gofr/rbac to use RBAC features")
		return
	}

	jwtExtractor := rbacRegistry.loader.NewJWTRoleExtractor(roleClaim)
	config.SetRoleExtractorFunc(jwtExtractor.ExtractRole)
	config.SetRequiresContainer(false)
}

func (a *App) applyRBACMiddleware(config RBACConfig) {
	if config.GetRequiresContainer() {
		a.httpServer.router.Use(rbacRegistry.middleware.Middleware(config, a.container))
	} else {
		a.httpServer.router.Use(rbacRegistry.middleware.Middleware(config))
	}
}

// RequireRole wraps a handler to require a specific role.
// This is a convenience wrapper that works with GoFr's Handler type.
//
// Note: This requires importing gofr.dev/pkg/gofr/rbac module.
func RequireRole(allowedRole string, handlerFunc Handler) Handler {
	if rbacRegistry.requireRole == nil {
		err := rbacRegistry.errAccessDenied
		if err == nil {
			err = errRBACModuleNotImportedAccess
		}

		return func(_ *Context) (any, error) {
			return nil, err
		}
	}

	rbacHandler := rbacRegistry.requireRole(allowedRole, func(ctx any) (any, error) {
		if gofrCtx, ok := ctx.(*Context); ok {
			return handlerFunc(gofrCtx)
		}

		return nil, rbacRegistry.errAccessDenied
	})

	return func(ctx *Context) (any, error) {
		return rbacHandler(ctx)
	}
}

// RequireAnyRole wraps a handler to require any of the specified roles.
// This is a convenience wrapper that works with GoFr's Handler type.
//
// Note: This requires importing gofr.dev/pkg/gofr/rbac module.
func RequireAnyRole(allowedRoles []string, handlerFunc Handler) Handler {
	if rbacRegistry.requireAnyRole == nil {
		err := rbacRegistry.errAccessDenied
		if err == nil {
			err = errRBACModuleNotImportedAccess
		}

		return func(_ *Context) (any, error) {
			return nil, err
		}
	}

	rbacHandler := rbacRegistry.requireAnyRole(allowedRoles, func(ctx any) (any, error) {
		if gofrCtx, ok := ctx.(*Context); ok {
			return handlerFunc(gofrCtx)
		}

		return nil, rbacRegistry.errAccessDenied
	})

	return func(ctx *Context) (any, error) {
		return rbacHandler(ctx)
	}
}

// RequirePermission wraps a handler to require a specific permission.
// This works with permission-based access control.
// The permissionConfig must be set in the RBAC config.
//
// Note: This requires importing gofr.dev/pkg/gofr/rbac module.
func RequirePermission(requiredPermission string, permissionConfig PermissionConfig, handlerFunc Handler) Handler {
	if rbacRegistry.requirePermission == nil {
		err := rbacRegistry.errPermissionDenied
		if err == nil {
			err = errRBACModuleNotImportedPermission
		}

		return func(_ *Context) (any, error) {
			return nil, err
		}
	}

	rbacHandler := rbacRegistry.requirePermission(requiredPermission, permissionConfig, func(ctx any) (any, error) {
		if gofrCtx, ok := ctx.(*Context); ok {
			return handlerFunc(gofrCtx)
		}

		return nil, rbacRegistry.errPermissionDenied
	})

	return func(ctx *Context) (any, error) {
		return rbacHandler(ctx)
	}
}
