package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
)

var (
	// errUnsupportedFormat is returned when the config file format is not supported.
	errUnsupportedFormat = errors.New("unsupported config file format")

	// ErrEndpointMissingPermissions is returned when an endpoint doesn't specify requiredPermissions and is not public.
	ErrEndpointMissingPermissions = errors.New("endpoint must specify requiredPermissions (or be public)")

	// errWildcardPatternNotSupported is returned when a wildcard pattern is used.
	errWildcardPatternNotSupported = errors.New("wildcard pattern '/*' is not supported, use mux patterns instead")

	// errRegexPatternNotSupported is returned when an old regex pattern is used.
	errRegexPatternNotSupported = errors.New("regex pattern '^...$' is not supported, use mux patterns instead")

	// errRegexIndicatorNotSupported is returned when regex indicators are used outside variable constraints.
	errRegexIndicatorNotSupported = errors.New("regex pattern is not supported, use mux patterns instead")
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
	// Path is the route path pattern using gorilla/mux syntax.
	// Examples:
	//   - "/api/users" (exact match)
	//   - "/api/users/{id}" (matches any single segment)
	//   - "/api/users/{id:[0-9]+}" (matches numeric IDs only)
	//   - "/api/{resource}" (single-level wildcard: matches /api/users, /api/posts)
	//   - "/api/{path:.*}" (multi-level wildcard: matches /api/users/123, /api/posts/comments)
	//   - "/api/{category}/posts" (middle variable: matches /api/tech/posts, /api/news/posts)
	// Only mux-style patterns are supported. Wildcards (/*) and regex (^...$) are not supported.
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
	Logger datasource.Logger `json:"-" yaml:"-"`

	// Metrics is the metrics instance for RBAC metrics
	// Set automatically by EnableRBAC
	Metrics container.Metrics `json:"-" yaml:"-"`

	// Tracer is the tracer instance for RBAC tracing
	// Set automatically by EnableRBAC
	Tracer trace.Tracer `json:"-" yaml:"-"`

	// Internal maps built from unified config (not in JSON/YAML)
	// These are populated by processUnifiedConfig()
	rolePermissionsMap    map[string][]string         `json:"-" yaml:"-"`
	endpointPermissionMap map[string][]string         `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: []permissions
	publicEndpointsMap    map[string]bool             `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: true if public
	endpointMap           map[string]*EndpointMapping `json:"-" yaml:"-"` // Key: "METHOD:/path", Value: endpoint object
	muxRouter             *mux.Router                 `json:"-" yaml:"-"` // Used for mux pattern matching
}

// LoadPermissions loads RBAC configuration from a JSON or YAML file.
// The file format is automatically detected based on the file extension.
// Supported formats: .json, .yaml, .yml.
// Dependencies (logger, metrics, tracer) are optional and can be set after loading.
func LoadPermissions(path string, logger datasource.Logger, metrics container.Metrics, tracer trace.Tracer) (*Config, error) {
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

	// Set dependencies
	config.Logger = logger
	config.Metrics = metrics
	config.Tracer = tracer

	// Initialize mux router for pattern matching
	// Use StrictSlash(false) to match the application router's behavior
	config.muxRouter = mux.NewRouter().StrictSlash(false)

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
	// Also validate that paths use mux patterns only (no wildcards or old regex)
	for i, endpoint := range c.Endpoints {
		if !endpoint.Public && len(endpoint.RequiredPermissions) == 0 {
			return fmt.Errorf("endpoint[%d]: %w: %s", i, ErrEndpointMissingPermissions, endpoint.Path)
		}

		// Validate path pattern
		if err := c.validateEndpointPath(endpoint.Path, i); err != nil {
			return err
		}
	}

	return nil
}

// validateEndpointPath validates that an endpoint path uses mux patterns only.
// Rejects wildcard patterns (/*) and old regex patterns (^...$).
func (c *Config) validateEndpointPath(path string, index int) error {
	if path == "" {
		return nil // Empty path is handled elsewhere
	}

	// Reject wildcard patterns
	if err := c.checkWildcardPattern(path, index); err != nil {
		return err
	}

	// Reject old regex patterns
	if err := c.checkRegexPattern(path, index); err != nil {
		return err
	}

	// Reject regex indicators outside of variable constraints
	if err := c.checkRegexIndicators(path, index); err != nil {
		return err
	}

	// Validate mux pattern syntax if it contains variables
	if isMuxPattern(path) {
		if err := validateMuxPattern(path); err != nil {
			return fmt.Errorf("endpoint[%d]: invalid mux pattern: %w", index, err)
		}
	}

	return nil
}

// checkWildcardPattern checks if path contains wildcard pattern.
func (*Config) checkWildcardPattern(path string, index int) error {
	if strings.Contains(path, "/*") {
		return fmt.Errorf("endpoint[%d]: %w: %s. Examples: /api/{resource} for single-level or /api/{path:.*} for multi-level",
			index, errWildcardPatternNotSupported, path)
	}

	return nil
}

// checkRegexPattern checks if path contains old regex pattern.
func (*Config) checkRegexPattern(path string, index int) error {
	if strings.HasPrefix(path, "^") || strings.HasSuffix(path, "$") {
		return fmt.Errorf("endpoint[%d]: %w: %s. Example: /api/users/{id:[0-9]+} instead of ^/api/users/\\d+$",
			index, errRegexPatternNotSupported, path)
	}

	return nil
}

// checkRegexIndicators checks if path contains regex indicators outside variable constraints.
func (*Config) checkRegexIndicators(path string, index int) error {
	if strings.Contains(path, "\\d") || strings.Contains(path, "\\w") || strings.Contains(path, "\\s") {
		// Only allow if it's inside a variable constraint like {id:[0-9]+}
		if !strings.Contains(path, "{") || !strings.Contains(path, ":") {
			return fmt.Errorf("endpoint[%d]: %w: %s. Example: /api/users/{id:[0-9]+}",
				index, errRegexIndicatorNotSupported, path)
		}
	}

	return nil
}

// processUnifiedConfig processes the unified Roles and Endpoints config
// and builds internal maps for efficient lookup.
// Config is read-only after initialization, so no mutex is needed.
func (c *Config) processUnifiedConfig() error {
	c.initializeMaps()
	c.buildRolePermissionsMap()

	return c.buildEndpointPermissionMap()
}

// initializeMaps initializes internal maps.
func (c *Config) initializeMaps() {
	c.rolePermissionsMap = make(map[string][]string)
	c.endpointPermissionMap = make(map[string][]string)
	c.publicEndpointsMap = make(map[string]bool)
	c.endpointMap = make(map[string]*EndpointMapping)
	// Use StrictSlash(false) to match the application router's behavior
	c.muxRouter = mux.NewRouter().StrictSlash(false)
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
		key := buildEndpointKey(endpoint, methodUpper)

		if err := c.storeEndpointMapping(endpoint, key, methodUpper); err != nil {
			return err
		}
	}

	return nil
}

// buildEndpointKey builds the key for an endpoint.
// Uses Path field which may contain mux patterns or exact paths.
func buildEndpointKey(endpoint *EndpointMapping, methodUpper string) string {
	pattern := endpoint.Path
	return fmt.Sprintf("%s:%s", methodUpper, pattern)
}

// storeEndpointMapping stores an endpoint mapping.
func (c *Config) storeEndpointMapping(endpoint *EndpointMapping, key, methodUpper string) error {
	// Store endpoint object for fast lookup
	c.endpointMap[key] = endpoint

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

// GetRolePermissions returns the permissions for a role.
// Config is read-only after initialization, so no mutex is needed.
func (c *Config) GetRolePermissions(role string) []string {
	return c.rolePermissionsMap[role]
}

// GetEndpointPermission returns the required permissions for an endpoint.
// Returns empty slice if endpoint is public or not found.
// Returns all required permissions (user needs ANY of them - OR logic).
// Config is read-only after initialization, so no mutex is needed.
func (c *Config) GetEndpointPermission(method, path string) ([]string, bool) {
	methodUpper := strings.ToUpper(method)
	key := fmt.Sprintf("%s:%s", methodUpper, path)

	// Try exact match first
	if perms, isPublic := c.checkExactMatch(key); isPublic || len(perms) > 0 {
		return perms, isPublic
	}

	// Try pattern and regex matching
	return c.checkPatternMatch(methodUpper, path)
}

// getExactEndpoint returns the endpoint for an exact key match (O(1) lookup).
// Config is read-only after initialization, so no mutex is needed.
func (c *Config) getExactEndpoint(key string) (*EndpointMapping, bool) {
	if endpoint, ok := c.endpointMap[key]; ok {
		isPublic := c.publicEndpointsMap[key]
		return endpoint, isPublic
	}

	return nil, false
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

// findEndpointByPattern finds an endpoint by pattern matching (wildcards/regex).
// Only used when exact match fails, so this is O(n) but only for patterns.
// Config is read-only after initialization, so no mutex is needed.
func (c *Config) findEndpointByPattern(methodUpper, path string) (*EndpointMapping, bool) {
	// Try pattern matching for wildcards/regex
	// Iterate over endpointMap to find matching patterns
	for key, endpoint := range c.endpointMap {
		if c.matchesKey(key, methodUpper, path) {
			isPublic := c.publicEndpointsMap[key]
			return endpoint, isPublic
		}
	}

	return nil, false
}

// checkPatternMatch checks for pattern and regex matches.
// Config is read-only after initialization, so no mutex is needed.
func (c *Config) checkPatternMatch(methodUpper, path string) (permissions []string, isPublic bool) {
	// Try pattern matching for wildcards
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

// matchesKey checks if a key matches the given method and path.
// Keys are built by buildEndpointKey which uses Path (may contain mux patterns).
// Uses mux Route.Match() for mux patterns, exact match for non-pattern paths.
func (c *Config) matchesKey(key, methodUpper, path string) bool {
	if !strings.HasPrefix(key, methodUpper+":") {
		return false
	}

	pattern := strings.TrimPrefix(key, methodUpper+":")

	// For exact paths (no variables), use string comparison
	if !isMuxPattern(pattern) {
		return pattern == path
	}

	// For mux patterns, use Route.Match() from endpoint_matcher
	return matchMuxPattern(pattern, methodUpper, path, c.muxRouter)
}
