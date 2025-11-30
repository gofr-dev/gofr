package rbac

import "net/http"

// RBACConfig is the minimal interface for RBAC configuration.
// This interface is defined in the rbac package to avoid cyclic imports.
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

	// GetRoleHeader returns the role header key if configured
	GetRoleHeader() string

	// SetRoleExtractorFunc sets the role extractor function
	SetRoleExtractorFunc(RoleExtractor)

	// SetLogger sets the logger instance
	SetLogger(any)

	// SetEnablePermissions enables permission-based access control
	SetEnablePermissions(bool)

	// InitializeMaps initializes empty maps if not present
	InitializeMaps()
}

// RoleExtractor extracts the user's role from the HTTP request.
type RoleExtractor func(req *http.Request, args ...any) (string, error)

// Options is an interface for RBAC configuration options.
// This follows the same pattern as service.Options for consistency.
type Options interface {
	AddOption(config RBACConfig) RBACConfig
}

