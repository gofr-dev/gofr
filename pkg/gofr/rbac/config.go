package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	// errUnsupportedFormat is returned when the config file format is not supported.
	errUnsupportedFormat = errors.New("unsupported config file format")

	// ErrEndpointMissingPermissions is returned when an endpoint doesn't specify requiredPermissions and is not public.
	ErrEndpointMissingPermissions = errors.New("endpoint must specify requiredPermissions (or be public)")
)

// RoleDefinition defines a role with its permissions and inheritance.
// Pure config-based: only role->permission mapping is supported.
type RoleDefinition struct {
	// Name is the role name (required)
	Name string `json:"name" yaml:"name"`

	// Permissions is a list of permissions for this role (format: "resource:action")
	// Example: ["users:read", "users:write"]
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`

	// InheritsFrom lists roles this role inherits permissions from
	// Example: ["viewer"] - editor inherits all viewer permissions
	InheritsFrom []string `json:"inheritsFrom,omitempty" yaml:"inheritsFrom,omitempty"`
}

// EndpointMapping defines authorization requirements for an API endpoint.
// Pure config-based: only route&method->permission mapping is supported.
// No direct route to role mapping - all authorization is permission-based.
type EndpointMapping struct {
	// Path is the route path pattern (supports wildcards like /api/*)
	// Example: "/api/users", "/api/users/*"
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Regex is a regular expression pattern for route matching (takes precedence over Path)
	// Example: "^/api/users/\\d+$"
	Regex string `json:"regex,omitempty" yaml:"regex,omitempty"`

	// Methods is a list of HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.)
	// Use ["*"] to match all methods
	// Example: ["GET", "POST"]
	Methods []string `json:"methods" yaml:"methods"`

	// RequiredPermissions is a list of permissions required to access this endpoint (format: "resource:action")
	// User needs to have ANY of these permissions (OR logic)
	// Example: ["users:read"] or ["users:read", "users:admin"]
	// This is checked against the role's permissions
	// REQUIRED: All endpoints must specify requiredPermissions (except public endpoints)
	RequiredPermissions []string `json:"requiredPermissions,omitempty" yaml:"requiredPermissions,omitempty"`

	// Public indicates this endpoint is publicly accessible (bypasses authorization)
	// Example: true for /health, /metrics endpoints
	Public bool `json:"public,omitempty" yaml:"public,omitempty"`
}

// Config represents the unified RBAC configuration structure.
type Config struct {
	// Roles defines all roles with their permissions and inheritance
	// This is the unified way to define roles (replaces RouteWithPermissions, RoleHierarchy)
	Roles []RoleDefinition `json:"roles,omitempty" yaml:"roles,omitempty"`

	// Endpoints maps API endpoints to authorization requirements
	// This is the unified way to define endpoint access (replaces RouteWithPermissions, OverRides)
	Endpoints []EndpointMapping `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`

	// RoleHeader specifies the HTTP header key for header-based role extraction
	// Example: "X-User-Role"
	// If set, role is extracted from this header
	RoleHeader string `json:"roleHeader,omitempty" yaml:"roleHeader,omitempty"`

	// JWTClaimPath specifies the JWT claim path for JWT-based role extraction
	// Examples: "role", "roles[0]", "permissions.role"
	// If set, role is extracted from JWT claims in request context
	JWTClaimPath string `json:"jwtClaimPath,omitempty" yaml:"jwtClaimPath,omitempty"`

	// ErrorHandler is called when authorization fails
	// If nil, default error response is sent
	ErrorHandler func(w http.ResponseWriter, r *http.Request, role, route string, err error)

	// Logger is the logger instance for audit logging
	// Set automatically by EnableRBAC - users don't need to configure this
	// Audit logging is automatically performed when RBAC is enabled
	Logger Logger `json:"-" yaml:"-"`

	// Internal maps built from unified config (not in JSON/YAML)
	// These are populated by processUnifiedConfig()
	rolePermissionsMap    map[string][]string `json:"-" yaml:"-"`
	endpointPermissionMap map[string]string   `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: permission
	publicEndpointsMap    map[string]bool     `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: true if public

	// Mutex for thread-safe access to maps (for future hot-reload support)
	mu sync.RWMutex `json:"-" yaml:"-"`
}

// SetLogger sets the logger for audit logging which asserts the Logger interface.
// This is called automatically by EnableRBAC - users don't need to configure this.
func (c *Config) SetLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.Logger = l
	}
}

// LoadPermissions loads RBAC configuration from a JSON or YAML file.
// The file format is automatically detected based on the file extension.
// Supported formats: .json, .yaml, .yml.
func LoadPermissions(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read RBAC config file %s: %w", path, err)
	}

	var config Config

	// Detect file format by extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", path, err)
		}
	case ".json", "":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml): %w", ext, errUnsupportedFormat)
	}

	// Process unified config to build internal maps
	if err := config.processUnifiedConfig(); err != nil {
		return nil, fmt.Errorf("failed to process unified config: %w", err)
	}

	return &config, nil
}

// processUnifiedConfig processes the unified Roles and Endpoints config
// and builds internal maps for efficient lookup.
func (c *Config) processUnifiedConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.initializeMaps()
	c.buildRolePermissionsMap()

	return c.buildEndpointPermissionMap()
}

// initializeMaps initializes internal maps.
func (c *Config) initializeMaps() {
	c.rolePermissionsMap = make(map[string][]string)
	c.endpointPermissionMap = make(map[string]string)
	c.publicEndpointsMap = make(map[string]bool)
}

// buildRolePermissionsMap builds the role permissions map from Roles.
func (c *Config) buildRolePermissionsMap() {
	for _, roleDef := range c.Roles {
		permissions := make([]string, len(roleDef.Permissions))
		copy(permissions, roleDef.Permissions)

		permissions = c.addInheritedPermissions(roleDef, permissions)
		c.rolePermissionsMap[roleDef.Name] = permissions
	}
}

// addInheritedPermissions adds inherited permissions to a role.
func (c *Config) addInheritedPermissions(roleDef RoleDefinition, permissions []string) []string {
	if len(roleDef.InheritsFrom) == 0 {
		return permissions
	}

	for _, inheritedRoleName := range roleDef.InheritsFrom {
		permissions = c.collectInheritedPermissions(inheritedRoleName, permissions)
	}

	return permissions
}

// collectInheritedPermissions collects permissions from an inherited role.
func (c *Config) collectInheritedPermissions(inheritedRoleName string, permissions []string) []string {
	for _, inheritedRole := range c.Roles {
		if inheritedRole.Name == inheritedRoleName {
			permissions = append(permissions, inheritedRole.Permissions...)

			if len(inheritedRole.InheritsFrom) > 0 {
				inheritedPerms := c.getEffectivePermissions(inheritedRoleName)
				permissions = append(permissions, inheritedPerms...)
			}

			break
		}
	}

	return permissions
}

// buildEndpointPermissionMap builds the endpoint permission map from Endpoints.
func (c *Config) buildEndpointPermissionMap() error {
	for _, endpoint := range c.Endpoints {
		methods := endpoint.Methods
		if len(methods) == 0 {
			methods = []string{"*"}
		}

		if err := c.processEndpointMethods(&endpoint, methods); err != nil {
			return err
		}
	}

	return nil
}

// processEndpointMethods processes methods for an endpoint.
func (c *Config) processEndpointMethods(endpoint *EndpointMapping, methods []string) error {
	for _, method := range methods {
		methodUpper := strings.ToUpper(method)
		key := c.buildEndpointKey(endpoint, methodUpper)

		if err := c.storeEndpointMapping(endpoint, key, methodUpper); err != nil {
			return err
		}
	}

	return nil
}

// buildEndpointKey builds the key for an endpoint.
func (*Config) buildEndpointKey(endpoint *EndpointMapping, methodUpper string) string {
	if endpoint.Regex != "" {
		return fmt.Sprintf("%s:%s", methodUpper, endpoint.Regex)
	}

	return fmt.Sprintf("%s:%s", methodUpper, endpoint.Path)
}

// storeEndpointMapping stores an endpoint mapping.
func (c *Config) storeEndpointMapping(endpoint *EndpointMapping, key, methodUpper string) error {
	if endpoint.Public {
		c.publicEndpointsMap[key] = true
		return nil
	}

	if len(endpoint.RequiredPermissions) == 0 {
		return fmt.Errorf("%w: %s %s", ErrEndpointMissingPermissions, methodUpper, endpoint.Path)
	}

	c.endpointPermissionMap[key] = endpoint.RequiredPermissions[0]

	return nil
}

// getEffectivePermissions recursively gets all permissions for a role including inherited ones.
func (c *Config) getEffectivePermissions(roleName string) []string {
	var permissions []string

	visited := make(map[string]bool)

	var collectPermissions func(string)

	collectPermissions = func(name string) {
		if visited[name] {
			return
		}

		visited[name] = true

		// Find role definition
		for _, roleDef := range c.Roles {
			if roleDef.Name == name {
				permissions = append(permissions, roleDef.Permissions...)
				// Recursively collect from inherited roles
				for _, inheritedName := range roleDef.InheritsFrom {
					collectPermissions(inheritedName)
				}

				break
			}
		}
	}

	collectPermissions(roleName)

	return permissions
}

// GetRolePermissions returns the permissions for a role (thread-safe).
func (c *Config) GetRolePermissions(role string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rolePermissionsMap[role]
}

// GetEndpointPermission returns the required permission for an endpoint (thread-safe).
// Returns empty string if endpoint is public or not found.
func (c *Config) GetEndpointPermission(method, path string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	methodUpper := strings.ToUpper(method)
	key := fmt.Sprintf("%s:%s", methodUpper, path)

	// Try exact match first
	if perm, isPublic := c.checkExactMatch(key); isPublic || perm != "" {
		return perm, isPublic
	}

	// Try pattern and regex matching
	return c.checkPatternMatch(methodUpper, path)
}

// checkExactMatch checks for an exact endpoint match.
func (c *Config) checkExactMatch(key string) (permission string, isPublic bool) {
	if public, ok := c.publicEndpointsMap[key]; ok && public {
		return "", true
	}

	if perm, ok := c.endpointPermissionMap[key]; ok {
		return perm, false
	}

	return "", false
}

// checkPatternMatch checks for pattern and regex matches.
func (c *Config) checkPatternMatch(methodUpper, path string) (permission string, isPublic bool) {
	// Try pattern matching for wildcards
	for key, perm := range c.endpointPermissionMap {
		if c.matchesKey(key, methodUpper, path) {
			return perm, false
		}
	}

	// Check public endpoints with pattern/regex
	for key := range c.publicEndpointsMap {
		if c.matchesKey(key, methodUpper, path) {
			return "", true
		}
	}

	return "", false
}

// matchesKey checks if a key matches the given method and path.
func (*Config) matchesKey(key, methodUpper, path string) bool {
	if !strings.HasPrefix(key, methodUpper+":") {
		return false
	}

	pattern := strings.TrimPrefix(key, methodUpper+":")

	// Try path pattern match
	if matchesPathPattern(pattern, path) {
		return true
	}

	// Try regex match
	matched, _ := regexp.MatchString(pattern, path)

	return matched
}

// matchesPathPattern checks if path matches pattern (supports wildcards).
func matchesPathPattern(pattern, path string) bool {
	if pattern == "" {
		return false
	}

	// Use path/filepath.Match for pattern matching
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Check prefix match for patterns ending with /*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}

	return false
}
