package gofr

import (
	"net/http"
)

// RBACConfig is the minimal interface for RBAC configuration.
// This interface matches rbac.RBACConfig to allow rbac.Config to implement it.
// The actual interface is defined in rbac package to avoid cyclic imports.
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

// PermissionConfig is the interface for permission-based access control.
type PermissionConfig interface {
	GetPermissions() map[string][]string
	GetRoutePermissionMap() map[string]string
	GetRolePermissions() map[string][]string
	GetRoutePermissionRules() []RoutePermissionRule
}

// RoutePermissionRule defines a rule for mapping routes to permissions.
type RoutePermissionRule struct {
	Methods    []string `json:"methods" yaml:"methods"`
	Path       string   `json:"path,omitempty" yaml:"path,omitempty"`
	Regex      string   `json:"regex,omitempty" yaml:"regex,omitempty"`
	Permission string   `json:"permission" yaml:"permission"`
}

// RBACOption is an interface for RBAC configuration options.
// This follows the same pattern as service.Options for consistency.
// The method name is AddOption to match service.Options pattern.
type RBACOption interface {
	AddOption(config RBACConfig) RBACConfig
}

// RBACHandlerFunc is a function type for RBAC handler wrappers.
type RBACHandlerFunc func(ctx any) (any, error)
