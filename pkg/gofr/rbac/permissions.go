package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
)

var (
	// ErrPermissionDenied is returned when a user doesn't have required permission.
	ErrPermissionDenied = errors.New("forbidden: permission denied")

	// errNoPermissionMapping is returned when no permission mapping is found for a route.
	errNoPermissionMapping = errors.New("no permission mapping found")
)

// PermissionConfig holds permission-based access control configuration.
type PermissionConfig struct {
	// Permissions maps permission names to allowed roles
	// Example: "users:read": ["admin", "editor", "viewer"]
	Permissions map[string][]string `json:"permissions" yaml:"permissions"`

	// RoutePermissionMap maps route patterns to required permissions
	// Format: "METHOD /path": "permission:action"
	// Example: "GET /api/users": "users:read"
	RoutePermissionMap map[string]string `json:"routePermissions" yaml:"routePermissions"`

	// DefaultPermission is used when route doesn't have explicit permission mapping
	DefaultPermission string `json:"defaultPermission,omitempty" yaml:"defaultPermission,omitempty"`
}

// HasPermission checks if the user's role has the specified permission.
func HasPermission(ctx context.Context, permission string, config *PermissionConfig) bool {
	if config == nil || config.Permissions == nil {
		return false
	}

	// Get user role from context
	role, _ := ctx.Value(userRole).(string)
	if role == "" {
		return false
	}

	// Get allowed roles for this permission
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

	return false
}

// GetRequiredPermission returns the required permission for a given route and method.
func GetRequiredPermission(method, route string, config *PermissionConfig) (string, error) {
	if config == nil || config.RoutePermissionMap == nil {
		if config != nil && config.DefaultPermission != "" {
			return config.DefaultPermission, nil
		}

		return "", fmt.Errorf("no permission mapping found for %s %s: %w", method, route, errNoPermissionMapping)
	}

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

	// Use default permission if configured
	if config.DefaultPermission != "" {
		return config.DefaultPermission, nil
	}

	return "", fmt.Errorf("no permission mapping found for %s %s: %w", method, route, errNoPermissionMapping)
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
