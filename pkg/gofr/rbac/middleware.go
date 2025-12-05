package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
)

type authMethod int

const userRole authMethod = 4

var (
	// ErrAccessDenied is returned when a user doesn't have required role/permission.
	ErrAccessDenied = errors.New("forbidden: access denied")

	// ErrRoleNotFound is returned when role cannot be extracted from request.
	ErrRoleNotFound = errors.New("unauthorized: role not found")

	// ErrJWTClaimsNotFound is returned when JWT claims are not found in request context.
	ErrJWTClaimsNotFound = errors.New("JWT claims not found in request context")

	// ErrEmptyClaimPath is returned when claim path is empty.
	ErrEmptyClaimPath = errors.New("empty claim path")

	// ErrClaimPathNotFound is returned when a claim path is not found in JWT claims.
	ErrClaimPathNotFound = errors.New("claim path not found")

	// ErrInvalidArrayNotation is returned when array notation is invalid.
	ErrInvalidArrayNotation = errors.New("invalid array notation")

	// ErrInvalidArrayIndex is returned when array index is invalid.
	ErrInvalidArrayIndex = errors.New("invalid array index")

	// ErrClaimKeyNotFound is returned when a claim key is not found.
	ErrClaimKeyNotFound = errors.New("claim key not found")

	// ErrClaimValueNotArray is returned when a claim value is not an array.
	ErrClaimValueNotArray = errors.New("claim value is not an array")

	// ErrArrayIndexOutOfBounds is returned when array index is out of bounds.
	ErrArrayIndexOutOfBounds = errors.New("array index out of bounds")

	// ErrInvalidClaimStructure is returned when claim structure is invalid.
	ErrInvalidClaimStructure = errors.New("invalid claim structure")
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
		return "", fmt.Errorf("%w", ErrJWTClaimsNotFound)
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
		return nil, fmt.Errorf("%w", ErrEmptyClaimPath)
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
		return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
	}

	return value, nil
}

// extractArrayClaim extracts a value from an array in JWT claims.
func extractArrayClaim(claims jwt.MapClaims, path string, idx int) (any, error) {
	key := path[:idx]
	arrayPath := path[idx:]

	// Extract array index
	if !strings.HasPrefix(arrayPath, "[") || !strings.HasSuffix(arrayPath, "]") {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArrayNotation, path)
	}

	indexStr := strings.Trim(arrayPath, "[]")

	var index int
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArrayIndex, indexStr)
	}

	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClaimKeyNotFound, key)
	}

	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrClaimValueNotArray, key)
	}

	if index < 0 || index >= len(arr) {
		return nil, fmt.Errorf("%w: %d (length: %d)", ErrArrayIndexOutOfBounds, index, len(arr))
	}

	return arr[index], nil
}

// extractNestedClaim extracts a value from nested structure in JWT claims.
func extractNestedClaim(claims jwt.MapClaims, path string) (any, error) {
	parts := strings.Split(path, ".")

	var current any = claims

	for i, part := range parts {
		isLast := i == len(parts)-1
		if isLast {
			return extractFinalClaimValue(current, part, path, parts, i)
		}

		// Navigate through nested structure
		current = navigateNestedClaim(current, part)
		if current == nil {
			return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, strings.Join(parts[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
}

// extractFinalClaimValue extracts the final value from a claim path.
func extractFinalClaimValue(current any, part, path string, parts []string, i int) (any, error) {
	if m, ok := current.(map[string]any); ok {
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
		}

		return value, nil
	}

	if m, ok := current.(jwt.MapClaims); ok {
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("%w: %s", ErrClaimPathNotFound, path)
		}

		return value, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrInvalidClaimStructure, strings.Join(parts[:i+1], "."))
}

// navigateNestedClaim navigates through nested claim structures.
func navigateNestedClaim(current any, part string) any {
	switch v := current.(type) {
	case map[string]any:
		return v[part]
	case jwt.MapClaims:
		return v[part]
	default:
		return nil
	}
}

// logAuditEvent logs authorization decisions for audit purposes.
// This is called automatically by the middleware when Logger is set.
// Users don't need to configure this - it uses the provided logger automatically.
func logAuditEvent(logger logging.Logger, r *http.Request, role, route string, allowed bool, reason string) {
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
