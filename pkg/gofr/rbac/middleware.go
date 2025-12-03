package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/pkg/gofr/http/middleware"
)

type authMethod int

const userRole authMethod = 4

var (
	// ErrAccessDenied is returned when a user doesn't have required role/permission.
	ErrAccessDenied = errors.New("forbidden: access denied")

	// ErrRoleNotFound is returned when role cannot be extracted from request.
	ErrRoleNotFound = errors.New("unauthorized: role not found")
)

// Middleware creates an HTTP middleware function that enforces RBAC authorization.
// It extracts the user's role and checks if the role is allowed for the requested route.
func Middleware(config *Config) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If config is nil, allow all requests (fail open)
			if config == nil {
				handler.ServeHTTP(w, r)
				return
			}

			route := r.URL.Path

			// Check if endpoint is public using unified Endpoints config
			endpoint, isPublic := getEndpointForRequest(r, config)
			if isPublic {
				handler.ServeHTTP(w, r)
				return
			}

			// If no endpoint match found, deny by default (fail secure)
			if endpoint == nil {
				handleAuthError(w, r, config, "", route, ErrAccessDenied)
				return
			}

			// Extract role using header-based or JWT-based extraction
			role, err := extractRole(r, config)
			if err != nil {
				handleAuthError(w, r, config, "", route, err)
				return
			}

			// Check authorization using unified endpoint-based authorization
			authorized, authReason := checkEndpointAuthorization(role, endpoint, config)
			if !authorized {
				handleAuthError(w, r, config, role, route, ErrAccessDenied)
				return
			}

			// Log audit event (always enabled when Logger is available)
			// Audit logging is automatically performed using GoFr's logger
			if config.Logger != nil {
				logAuditEvent(config.Logger, r, role, route, true, authReason)
			}

			// Store role in context and continue
			ctx := context.WithValue(r.Context(), userRole, role)
			handler.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// handleAuthError handles authorization errors with custom error handler or default response.
func handleAuthError(w http.ResponseWriter, r *http.Request, config *Config, role, route string, err error) {
	// Log audit event (always enabled when Logger is available)
	// Audit logging is automatically performed using GoFr's logger
	if config.Logger != nil {
		reason := "access denied"

		if errors.Is(err, ErrRoleNotFound) {
			reason = "role not found"
		}

		logAuditEvent(config.Logger, r, role, route, false, reason)
	}

	// Use custom error handler if provided
	if config.ErrorHandler != nil {
		config.ErrorHandler(w, r, role, route, err)
		return
	}

	// Default error handling
	if errors.Is(err, ErrRoleNotFound) {
		http.Error(w, "Unauthorized: Missing or invalid role", http.StatusUnauthorized)
		return
	}

	http.Error(w, "Forbidden: Access denied", http.StatusForbidden)
}

// extractRole extracts the user's role from the request.
// Supports header-based extraction (via RoleHeader) or JWT-based extraction (via JWTClaimPath).
// Precedence: JWT takes precedence over header (JWT is more secure).
// No default role is supported - role must be explicitly provided.
func extractRole(r *http.Request, config *Config) (string, error) {
	// Try JWT-based extraction first (takes precedence - more secure)
	if config.JWTClaimPath != "" {
		role, err := extractRoleFromJWT(r, config.JWTClaimPath)
		if err == nil && role != "" {
			return role, nil
		}
		// If JWT extraction fails but JWTClaimPath is set, don't fall back to header
		// This ensures JWT is the only method when configured
		return "", ErrRoleNotFound
	}

	// Try header-based extraction (only if JWT is not configured)
	if config.RoleHeader != "" {
		role := r.Header.Get(config.RoleHeader)
		if role != "" {
			return role, nil
		}
	}

	// No role found - no default role supported
	return "", ErrRoleNotFound
}

// extractRoleFromJWT extracts the role from JWT claims in the request context.
// It uses the JWTClaimPath from config to navigate the claim structure.
func extractRoleFromJWT(r *http.Request, claimPath string) (string, error) {
	// Get JWT claims from context (set by OAuth middleware)
	claims, ok := r.Context().Value(middleware.JWTClaim).(jwt.MapClaims)
	if !ok || claims == nil {
		return "", fmt.Errorf("JWT claims not found in request context")
	}

	// Extract role using the configured claim path
	role, err := extractClaimValue(claims, claimPath)
	if err != nil {
		return "", fmt.Errorf("failed to extract role from JWT: %w", err)
	}

	// Convert to string
	roleStr, ok := role.(string)
	if !ok {
		// Try to convert if it's not a string
		return fmt.Sprintf("%v", role), nil
	}

	return roleStr, nil
}

// extractClaimValue extracts a value from JWT claims using a dot-notation or array notation path.
// Examples:
//   - "role" -> claims["role"]
//   - "roles[0]" -> claims["roles"].([]any)[0]
//   - "permissions.role" -> claims["permissions"].(map[string]any)["role"]
func extractClaimValue(claims jwt.MapClaims, path string) (any, error) {
	if path == "" {
		return nil, fmt.Errorf("empty claim path")
	}

	// Handle array notation: "roles[0]"
	if idx := strings.Index(path, "["); idx != -1 {
		return extractArrayClaim(claims, path, idx)
	}

	// Handle dot notation: "permissions.role"
	if strings.Contains(path, ".") {
		return extractNestedClaim(claims, path)
	}

	// Simple key lookup
	value, ok := claims[path]
	if !ok {
		return nil, fmt.Errorf("claim path not found: %s", path)
	}

	return value, nil
}

// extractArrayClaim extracts a value from an array in JWT claims.
func extractArrayClaim(claims jwt.MapClaims, path string, idx int) (any, error) {
	key := path[:idx]
	arrayPath := path[idx:]

	// Extract array index
	if !strings.HasPrefix(arrayPath, "[") || !strings.HasSuffix(arrayPath, "]") {
		return nil, fmt.Errorf("invalid array notation: %s", path)
	}

	indexStr := strings.Trim(arrayPath, "[]")
	var index int
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		return nil, fmt.Errorf("invalid array index: %s", indexStr)
	}

	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("claim key not found: %s", key)
	}

	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("claim value is not an array: %s", key)
	}

	if index < 0 || index >= len(arr) {
		return nil, fmt.Errorf("array index out of bounds: %d (length: %d)", index, len(arr))
	}

	return arr[index], nil
}

// extractNestedClaim extracts a value from nested structure in JWT claims.
func extractNestedClaim(claims jwt.MapClaims, path string) (any, error) {
	parts := strings.Split(path, ".")
	var current any = claims

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - return the value
			if m, ok := current.(map[string]any); ok {
				value, exists := m[part]
				if !exists {
					return nil, fmt.Errorf("claim path not found: %s", path)
				}
				return value, nil
			}
			if m, ok := current.(jwt.MapClaims); ok {
				value, exists := m[part]
				if !exists {
					return nil, fmt.Errorf("claim path not found: %s", path)
				}
				return value, nil
			}
			return nil, fmt.Errorf("invalid claim structure: %s", strings.Join(parts[:i+1], "."))
		}

		// Navigate through nested structure
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else if m, ok := current.(jwt.MapClaims); ok {
			current = m[part]
		} else {
			return nil, fmt.Errorf("invalid claim structure: %s", strings.Join(parts[:i+1], "."))
		}

		if current == nil {
			return nil, fmt.Errorf("claim path not found: %s", strings.Join(parts[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("claim path not found: %s", path)
}

// logAuditEvent logs authorization decisions for audit purposes.
// This is called automatically by the middleware when Logger is set.
// Users don't need to configure this - it uses the provided logger automatically.
func logAuditEvent(logger Logger, r *http.Request, role, route string, allowed bool, reason string) {
	if logger == nil {
		return // Skip logging if no logger provided
	}

	status := "denied"
	if allowed {
		status = "allowed"
	}

	logger.Infof("[RBAC Audit] %s %s - Role: %s - Route: %s - %s - Reason: %s",
		r.Method, r.URL.Path, role, route, status, reason)
}
