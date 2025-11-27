package gofr

import (
	"net/http"
)

// RBACConfig is the minimal interface for RBAC configuration.
// This allows RBAC to be an optional module - users import the RBAC module
// which implements this interface.
type RBACConfig interface {
	// GetRouteWithPermissions returns the route-to-roles mapping
	GetRouteWithPermissions() map[string][]string

	// GetRoleExtractorFunc returns the role extractor function
	GetRoleExtractorFunc() RoleExtractor

	// GetPermissionConfig returns permission configuration if enabled
	// Returns nil if permissions are not configured
	// The returned value should implement PermissionConfig interface
	GetPermissionConfig() any

	// GetOverRides returns route overrides
	GetOverRides() map[string]bool

	// GetLogger returns the logger instance
	GetLogger() any

	// GetRequiresContainer returns whether container access is needed
	GetRequiresContainer() bool

	// SetRoleExtractorFunc sets the role extractor function
	SetRoleExtractorFunc(RoleExtractor)

	// SetLogger sets the logger instance
	SetLogger(any)

	// SetRequiresContainer sets whether container access is needed
	SetRequiresContainer(bool)

	// SetEnablePermissions enables permission-based access control
	SetEnablePermissions(bool)

	// InitializeMaps initializes empty maps if not present
	InitializeMaps()
}

// RoleExtractor extracts the user's role from the HTTP request.
type RoleExtractor func(req *http.Request, args ...any) (string, error)

// PermissionConfig is the interface for permission-based access control.
type PermissionConfig interface {
	GetPermissions() map[string][]string
	GetRoutePermissionMap() map[string]string
}

// RBACLoader loads RBAC configuration from files.
type RBACLoader interface {
	// LoadPermissions loads configuration from a file
	LoadPermissions(file string) (RBACConfig, error)

	// NewConfigLoaderWithLogger creates a config loader
	NewConfigLoaderWithLogger(file string, logger any) (ConfigLoader, error)

	// NewJWTRoleExtractor creates a JWT-based role extractor
	NewJWTRoleExtractor(claim string) JWTRoleExtractor
}

// JWTRoleExtractor extracts role from JWT claims.
type JWTRoleExtractor interface {
	ExtractRole(req *http.Request, args ...any) (string, error)
}

// ConfigLoader manages RBAC configuration loading.
type ConfigLoader interface {
	GetConfig() RBACConfig
}

// RBACMiddleware creates HTTP middleware for RBAC authorization.
type RBACMiddleware interface {
	// Middleware creates an HTTP middleware function
	Middleware(config RBACConfig, args ...any) func(http.Handler) http.Handler
}

// RBACHandlerFunc is a function type for RBAC handler wrappers.
type RBACHandlerFunc func(ctx any) (any, error)

// rbacRegistry holds registered RBAC functions.
// RBAC module registers its implementations here at init time.
//
//nolint:gochecknoglobals // Registry pattern requires global variable for module registration
var rbacRegistry struct {
	loader              RBACLoader
	middleware          RBACMiddleware
	requireRole         func(string, RBACHandlerFunc) RBACHandlerFunc
	requireAnyRole      func([]string, RBACHandlerFunc) RBACHandlerFunc
	requirePermission   func(string, PermissionConfig, RBACHandlerFunc) RBACHandlerFunc
	errAccessDenied     error
	errPermissionDenied error
}

// RegisterRBAC registers RBAC implementations.
// This is called by the RBAC module at init time.
func RegisterRBAC(loader RBACLoader, middleware RBACMiddleware,
	requireRole func(string, RBACHandlerFunc) RBACHandlerFunc,
	requireAnyRole func([]string, RBACHandlerFunc) RBACHandlerFunc,
	requirePermission func(string, PermissionConfig, RBACHandlerFunc) RBACHandlerFunc,
	errAccessDenied, errPermissionDenied error) {
	rbacRegistry.loader = loader
	rbacRegistry.middleware = middleware
	rbacRegistry.requireRole = requireRole
	rbacRegistry.requireAnyRole = requireAnyRole
	rbacRegistry.requirePermission = requirePermission
	rbacRegistry.errAccessDenied = errAccessDenied
	rbacRegistry.errPermissionDenied = errPermissionDenied
}
