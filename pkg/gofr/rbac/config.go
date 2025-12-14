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

	"go.opentelemetry.io/otel/trace"
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
	// Path is the route path pattern (supports wildcards like /api/* or regex patterns)
	// Examples:
	//   - "/api/users" (exact match)
	//   - "/api/users/*" (wildcard pattern)
	//   - "^/api/users/\\d+$" (regex pattern - automatically detected)
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

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

	// Metrics is the metrics instance for RBAC metrics
	// Set automatically by EnableRBAC
	Metrics Metrics `json:"-" yaml:"-"`

	// Tracer is the tracer instance for RBAC tracing
	// Set automatically by EnableRBAC
	Tracer trace.Tracer `json:"-" yaml:"-"`

	// Internal maps built from unified config (not in JSON/YAML)
	// These are populated by processUnifiedConfig()
	rolePermissionsMap    map[string][]string       `json:"-" yaml:"-"`
	endpointPermissionMap map[string][]string       `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: []permissions
	publicEndpointsMap    map[string]bool           `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: true if public
	compiledRegexMap      map[string]*regexp.Regexp `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: compiled regex

	// Mutex for thread-safe access to maps (for future hot-reload support)
	mu sync.RWMutex `json:"-" yaml:"-"`
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

	// Validate config before processing
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid RBAC config: %w", err)
	}

	// Process unified config to build internal maps
	if err := config.processUnifiedConfig(); err != nil {
		return nil, fmt.Errorf("failed to process unified config: %w", err)
	}

	return &config, nil
}

// validate validates the RBAC configuration.
func (c *Config) validate() error {
	// Validate endpoints: non-public endpoints must have RequiredPermissions
	for i, endpoint := range c.Endpoints {
		if !endpoint.Public && len(endpoint.RequiredPermissions) == 0 {
			return fmt.Errorf("endpoint[%d]: %w: %s", i, ErrEndpointMissingPermissions, endpoint.Path)
		}
	}

	return nil
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
	c.endpointPermissionMap = make(map[string][]string)
	c.publicEndpointsMap = make(map[string]bool)
	c.compiledRegexMap = make(map[string]*regexp.Regexp)
}

// buildRolePermissionsMap builds the role permissions map from Roles.
// Uses getEffectivePermissions() for consistent inheritance logic.
func (c *Config) buildRolePermissionsMap() {
	for _, roleDef := range c.Roles {
		// Use getEffectivePermissions() for consistent inheritance handling
		permissions := c.getEffectivePermissions(roleDef.Name)
		c.rolePermissionsMap[roleDef.Name] = permissions
	}
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
// Uses Path field which may contain either a path pattern or a regex pattern.
// Note: This is called during processUnifiedConfig which already holds a lock,
// so we can directly access compiledRegexMap without acquiring another lock.
func (c *Config) buildEndpointKey(endpoint *EndpointMapping, methodUpper string) string {
	pattern := endpoint.Path

	// If pattern looks like a regex, precompile it for performance
	// Store with pattern as key (not full METHOD:pattern) since matchesKey
	// uses pattern directly for lookup
	if isRegexPattern(pattern) {
		// We're already holding a lock from processUnifiedConfig, so access map directly
		if _, exists := c.compiledRegexMap[pattern]; !exists {
			if compiled, err := regexp.Compile(pattern); err == nil {
				c.compiledRegexMap[pattern] = compiled
			}
		}
	}

	return fmt.Sprintf("%s:%s", methodUpper, pattern)
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

	// Store all required permissions (not just the first one)
	permissions := make([]string, len(endpoint.RequiredPermissions))
	copy(permissions, endpoint.RequiredPermissions)
	c.endpointPermissionMap[key] = permissions

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

// GetEndpointPermission returns the required permissions for an endpoint (thread-safe).
// Returns empty slice if endpoint is public or not found.
// Returns all required permissions (user needs ANY of them - OR logic).
func (c *Config) GetEndpointPermission(method, path string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	methodUpper := strings.ToUpper(method)
	key := fmt.Sprintf("%s:%s", methodUpper, path)

	// Try exact match first
	if perms, isPublic := c.checkExactMatch(key); isPublic || len(perms) > 0 {
		return perms, isPublic
	}

	// Try pattern and regex matching
	return c.checkPatternMatch(methodUpper, path)
}

// checkExactMatch checks for an exact endpoint match.
func (c *Config) checkExactMatch(key string) (permissions []string, isPublic bool) {
	if public, ok := c.publicEndpointsMap[key]; ok && public {
		return nil, true
	}

	if perms, ok := c.endpointPermissionMap[key]; ok {
		return perms, false
	}

	return nil, false
}

// checkPatternMatch checks for pattern and regex matches.
// Note: This is called while already holding RLock from GetEndpointPermission.
// matchesKey will acquire another RLock which is safe (read locks can be nested).
func (c *Config) checkPatternMatch(methodUpper, path string) (permissions []string, isPublic bool) {
	// Try pattern matching for wildcards
	// Note: We iterate over the map while holding RLock, which is safe for read-only operations
	for key, perms := range c.endpointPermissionMap {
		if c.matchesKey(key, methodUpper, path) {
			return perms, false
		}
	}

	// Check public endpoints with pattern/regex
	for key := range c.publicEndpointsMap {
		if c.matchesKey(key, methodUpper, path) {
			return nil, true
		}
	}

	return nil, false
}

// isRegexPattern detects if a pattern is likely a regex.
// Checks for common regex indicators: starts with ^, ends with $, or contains regex special chars.
func isRegexPattern(pattern string) bool {
	return strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") ||
		strings.Contains(pattern, "\\d") || strings.Contains(pattern, "\\w") ||
		strings.Contains(pattern, "\\s") || strings.Contains(pattern, "[") ||
		strings.Contains(pattern, "(") || strings.Contains(pattern, "?")
}

// matchesKey checks if a key matches the given method and path.
// Keys are built by buildEndpointKey which uses Path (may contain regex patterns).
// If pattern looks like a regex (starts with ^ or contains regex special chars), use regex matching exclusively.
// Otherwise, use path pattern matching exclusively (no fallback to regex).
func (c *Config) matchesKey(key, methodUpper, path string) bool {
	if !strings.HasPrefix(key, methodUpper+":") {
		return false
	}

	pattern := strings.TrimPrefix(key, methodUpper+":")

	if isRegexPattern(pattern) {
		// Try to use precompiled regex for better performance
		// Note: We may already be holding RLock from checkPatternMatch, but nested read locks are safe
		c.mu.RLock()
		compiled, exists := c.compiledRegexMap[pattern]
		c.mu.RUnlock()

		if exists {
			return compiled.MatchString(path)
		}

		// Fallback to runtime compilation if not precompiled (shouldn't happen if config was processed correctly)
		// Compile with a timeout-safe approach - use MustCompile in a recover block or just MatchString
		matched, err := regexp.MatchString(pattern, path)
		if err != nil {
			// Invalid regex - no match
			return false
		}

		return matched
	}

	// For path patterns, only try path matching
	// Since buildEndpointKey uses Path (which may contain regex patterns),
	// if it's not detected as regex, it's a path pattern from buildEndpointKey
	return matchesPathPattern(pattern, path)
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
