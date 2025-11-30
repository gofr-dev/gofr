package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
)

var (
	// ErrPermissionDenied is returned when a user doesn't have required permission.
	ErrPermissionDenied = errors.New("forbidden: permission denied")

	// errNoPermissionMapping is returned when no permission mapping is found for a route.
	errNoPermissionMapping = errors.New("no permission mapping found")
)

// RoutePermissionRule defines a rule for mapping routes to permissions.
type RoutePermissionRule struct {
	// Methods is a list of HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.)
	// If empty or contains "*", matches all methods
	Methods []string `json:"methods" yaml:"methods"`

	// Path is a path pattern (supports wildcards like /api/*)
	// Used when Regex is empty
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Regex is a regular expression pattern for route matching
	// Takes precedence over Path if both are provided
	Regex string `json:"regex,omitempty" yaml:"regex,omitempty"`

	// Permission is the required permission for matching routes
	Permission string `json:"permission" yaml:"permission"`

	// compiledRegex is the compiled regex (internal, not in JSON/YAML)
	compiledRegex *regexp.Regexp `json:"-" yaml:"-"`
}

// PermissionConfig holds permission-based access control configuration.
type PermissionConfig struct {
	// RolePermissions maps roles to their permissions (ROLE-CENTRIC)
	// Example: "admin": ["users:read", "users:write", "users:delete"]
	RolePermissions map[string][]string `json:"rolePermissions" yaml:"rolePermissions"`

	// RoutePermissionRules is a list of rules for mapping routes to permissions
	// More flexible than RoutePermissionMap - supports multiple methods, regex, etc.
	RoutePermissionRules []RoutePermissionRule `json:"routePermissionRules,omitempty" yaml:"routePermissionRules,omitempty"`

	// RoutePermissionMap is the legacy format (for backward compatibility)
	// Format: "METHOD /path": "permission:action"
	// Example: "GET /api/users": "users:read"
	// Deprecated: Use RoutePermissionRules instead
	RoutePermissionMap map[string]string `json:"routePermissions,omitempty" yaml:"routePermissions,omitempty"`

	// Permissions is the legacy permission-centric format (for backward compatibility)
	// Example: "users:read": ["admin", "editor", "viewer"]
	// Deprecated: Use RolePermissions instead
	Permissions map[string][]string `json:"permissions,omitempty" yaml:"permissions,omitempty"`

	// DefaultPermission is used when route doesn't have explicit permission mapping
	DefaultPermission string `json:"defaultPermission,omitempty" yaml:"defaultPermission,omitempty"`
}

// Implement PermissionConfig interface methods

// GetPermissions returns the legacy permissions map (for backward compatibility).
func (p *PermissionConfig) GetPermissions() map[string][]string {
	return p.Permissions
}

// GetRoutePermissionMap returns the legacy route-to-permission mapping (for backward compatibility).
func (p *PermissionConfig) GetRoutePermissionMap() map[string]string {
	return p.RoutePermissionMap
}

// GetRolePermissions returns the role-centric permissions map.
func (p *PermissionConfig) GetRolePermissions() map[string][]string {
	return p.RolePermissions
}

// GetRoutePermissionRules returns the structured route permission rules.
func (p *PermissionConfig) GetRoutePermissionRules() []RoutePermissionRule {
	return p.RoutePermissionRules
}

// CompileRoutePermissionRules compiles regex patterns in RoutePermissionRules.
func (p *PermissionConfig) CompileRoutePermissionRules() error {
	for i := range p.RoutePermissionRules {
		rule := &p.RoutePermissionRules[i]
		if rule.Regex != "" {
			regex, err := regexp.Compile(rule.Regex)
			if err != nil {
				return fmt.Errorf("invalid regex pattern %q: %w", rule.Regex, err)
			}
			rule.compiledRegex = regex
		}
	}
	return nil
}

// HasPermission checks if the user's role has the specified permission.
// Uses role-centric lookup (preferred) or falls back to permission-centric (legacy).
func HasPermission(ctx context.Context, permission string, config *PermissionConfig) bool {
	if config == nil {
		return false
	}

	// Get user role from context
	role, _ := ctx.Value(userRole).(string)
	if role == "" {
		return false
	}

	// Try role-centric lookup first (preferred)
	if config.RolePermissions != nil {
		rolePerms, exists := config.RolePermissions[role]
		if exists {
			// Check if permission is in role's permissions
			for _, rolePerm := range rolePerms {
				if rolePerm == permission || rolePerm == "*" {
					return true
				}
			}
		}
	}

	// Fallback to permission-centric lookup (legacy)
	if config.Permissions != nil {
		allowedRoles, exists := config.Permissions[permission]
		if !exists {
			return false
		}

		// Check if user's role is in allowed roles
		for _, allowedRole := range allowedRoles {
			if allowedRole == role || allowedRole == "*" {
				return true
			}
		}
	}

	return false
}

// GetRequiredPermission returns the required permission for a given route and method.
func GetRequiredPermission(method, route string, config *PermissionConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("no permission mapping found for %s %s: %w", method, route, errNoPermissionMapping)
	}

	// First, try RoutePermissionRules (new structured format)
	if len(config.RoutePermissionRules) > 0 {
		if permission := matchRoutePermissionRules(method, route, config.RoutePermissionRules); permission != "" {
			return permission, nil
		}
	}

	// Fallback to RoutePermissionMap (legacy format)
	if config.RoutePermissionMap != nil {
		// Try exact match: "GET /api/users"
		key := fmt.Sprintf("%s %s", method, route)
		if permission, exists := config.RoutePermissionMap[key]; exists {
			return permission, nil
		}

		// Try pattern matching with wildcards
		for pattern, permission := range config.RoutePermissionMap {
			if matchesRoutePattern(pattern, method, route) {
				return permission, nil
			}
		}
	}

	// Use default permission if configured
	if config.DefaultPermission != "" {
		return config.DefaultPermission, nil
	}

	return "", fmt.Errorf("no permission mapping found for %s %s: %w", method, route, errNoPermissionMapping)
}

// matchRoutePermissionRules matches a route against RoutePermissionRules.
func matchRoutePermissionRules(method, route string, rules []RoutePermissionRule) string {
	for i := range rules {
		rule := &rules[i]
		// Check method match
		if !matchesMethod(method, rule.Methods) {
			continue
		}

		// Check route match (regex takes precedence)
		if rule.Regex != "" {
			if rule.compiledRegex == nil {
				// Compile regex on first use
				regex, err := regexp.Compile(rule.Regex)
				if err != nil {
					continue // Skip invalid regex
				}
				rule.compiledRegex = regex
			}
			if rule.compiledRegex.MatchString(route) {
				return rule.Permission
			}
		} else if rule.Path != "" {
			// Use path pattern matching (supports wildcards)
			if matchesPathPattern(rule.Path, route) {
				return rule.Permission
			}
		}
	}
	return ""
}

// matchesMethod checks if the HTTP method matches any of the allowed methods.
func matchesMethod(method string, allowedMethods []string) bool {
	// Empty methods or "*" means all methods
	if len(allowedMethods) == 0 {
		return true
	}

	for _, m := range allowedMethods {
		if m == "*" || strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// matchesPathPattern checks if route matches path pattern (supports wildcards).
func matchesPathPattern(pattern, route string) bool {
	// Use path/filepath.Match for pattern matching
	matched, _ := path.Match(pattern, route)
	if matched {
		return true
	}

	// Also check prefix match for patterns ending with /*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return route == prefix || strings.HasPrefix(route, prefix+"/")
	}

	return false
}

// matchesRoutePattern checks if a route pattern matches the given method and route.
// Supports wildcards: "GET /api/*" matches "GET /api/users".
func matchesRoutePattern(pattern, method, route string) bool {
	// Split pattern into method and path
	const expectedParts = 2

	parts := strings.SplitN(pattern, " ", expectedParts)

	if len(parts) != expectedParts {
		return false
	}

	patternMethod := parts[0]
	patternPath := parts[1]

	// Check method match (supports wildcard)
	if patternMethod != "*" && patternMethod != method {
		return false
	}

	// Use path/filepath.Match for path pattern matching
	matched, _ := path.Match(patternPath, route)

	return matched
}

// CheckPermission checks if the user has the required permission for the route.
func CheckPermission(req *http.Request, config *PermissionConfig) error {
	if config == nil {
		return ErrPermissionDenied
	}

	// Get required permission for this route
	permission, err := GetRequiredPermission(req.Method, req.URL.Path, config)
	if err != nil {
		// If no permission mapping found and no default, deny access
		return ErrPermissionDenied
	}

	// Check if user has the permission
	if !HasPermission(req.Context(), permission, config) {
		return ErrPermissionDenied
	}

	return nil
}

// RequirePermission wraps a handler to require a specific permission.
// Note: For GoFr applications, use gofr.RequirePermission() instead for better type safety.
func RequirePermission(requiredPermission string, config *PermissionConfig, handlerFunc HandlerFunc) HandlerFunc {
	return func(ctx any) (any, error) {
		reqCtx := extractContextFromCtx(ctx)

		if !HasPermission(reqCtx, requiredPermission, config) {
			return nil, ErrPermissionDenied
		}

		return handlerFunc(ctx)
	}
}

// extractContextFromCtx extracts context.Context from the given context value.
func extractContextFromCtx(ctx any) context.Context {
	type contextValueGetter interface {
		Value(key any) any
	}

	ctxWithValue, ok := ctx.(contextValueGetter)
	if !ok {
		return context.Background()
	}

	// Try to extract context.Context from GoFr context
	if gofrCtx, ok := ctx.(interface{ Context() context.Context }); ok {
		return gofrCtx.Context()
	}

	// Fallback: create a context with role value
	roleVal := ctxWithValue.Value(userRole)
	if roleVal != nil {
		return context.WithValue(context.Background(), userRole, roleVal)
	}

	return context.Background()
}
