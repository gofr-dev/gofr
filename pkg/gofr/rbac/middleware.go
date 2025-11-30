package rbac

import (
	"context"
	"errors"
	"net/http"
)

type authMethod int

const userRole authMethod = 4

const (
	authReasonPermissionBased    = "permission-based"
	authReasonRoleBasedHierarchy = "role-based (with hierarchy)"
	authReasonRoleBased          = "role-based"
)

var (
	// ErrAccessDenied is returned when a user doesn't have required role/permission.
	ErrAccessDenied = errors.New("forbidden: access denied")

	// ErrRoleNotFound is returned when role cannot be extracted from request.
	ErrRoleNotFound = errors.New("unauthorized: role not found")
)

// auditLogger is an internal logger for authorization decisions.
// Audit logging is automatically performed using GoFr's logger - users don't need to configure this.
type auditLogger struct{}

// logAccess logs an authorization decision using the logger.
func (*auditLogger) logAccess(logger Logger, req *http.Request, role, route string, allowed bool, reason string) {
	if logger == nil {
		return // Skip logging if no logger provided
	}

	status := "denied"
	if allowed {
		status = "allowed"
	}

	logger.Infof("[RBAC Audit] %s %s - Role: %s - Route: %s - %s - Reason: %s",
		req.Method, req.URL.Path, role, route, status, reason)
}

// Middleware creates an HTTP middleware function that enforces RBAC authorization.
// It extracts the user's role and checks if the role is allowed for the requested route.
//
// Users should use app.EnableRBAC() with options instead of calling this function directly.
func Middleware(config *Config) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If config is nil, allow all requests (fail open)
			if config == nil {
				handler.ServeHTTP(w, r)
				return
			}

			route := r.URL.Path

			// Check if route is overridden (public access)
			if config.OverRides != nil && config.OverRides[route] {
				handler.ServeHTTP(w, r)
				return
			}

			// Extract role
			role, err := extractRole(r, config)
			if err != nil {
				handleAuthError(w, r, config, "", route, err)
				return
			}

			// Check authorization
			authorized, authReason := checkAuthorization(r, role, route, config)
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

// extractRole extracts the user's role from the request using the configured extractor.
func extractRole(r *http.Request, config *Config) (string, error) {
	if config.RoleExtractorFunc == nil {
		if config.DefaultRole != "" {
			return config.DefaultRole, nil
		}

		return "", ErrRoleNotFound
	}

	// Role extractor is called without any args (no container access)
	role, err := config.RoleExtractorFunc(r)
	if err != nil {
		if config.DefaultRole != "" {
			return config.DefaultRole, nil
		}

		return "", ErrRoleNotFound
	}

	// If role is empty, use default role if available
	if role == "" && config.DefaultRole != "" {
		return config.DefaultRole, nil
	}

	// If role is still empty and no default role, return error
	if role == "" {
		return "", ErrRoleNotFound
	}

	return role, nil
}

// checkAuthorization checks if the user is authorized to access the route.
func checkAuthorization(r *http.Request, role, route string, config *Config) (authorized bool, reason string) {
	// Check permission-based access if enabled
	if config.EnablePermissions && config.PermissionConfig != nil {
		reqCtx := context.WithValue(r.Context(), userRole, role)
		reqWithRole := r.WithContext(reqCtx)

		if err := CheckPermission(reqWithRole, config.PermissionConfig); err == nil {
			return true, authReasonPermissionBased
		}
	}

	// Check role-based access
	if len(config.RoleHierarchy) > 0 {
		hierarchy := NewRoleHierarchy(config.RoleHierarchy)

		if IsRoleAllowedWithHierarchy(role, route, config, hierarchy) {
			return true, authReasonRoleBasedHierarchy
		}
	}

	if isRoleAllowed(role, route, config) {
		return true, authReasonRoleBased
	}

	return false, ""
}

// logAuditEvent logs authorization decisions for audit purposes.
// This is called automatically by the middleware when Logger is set.
// Users don't need to configure this - it uses the provided logger automatically.
func logAuditEvent(logger Logger, r *http.Request, role, route string, allowed bool, reason string) {
	auditLogger := &auditLogger{}
	auditLogger.logAccess(logger, r, role, route, allowed, reason)
}

// HandlerFunc is a function type that matches GoFr's handler signature.
// This avoids import cycle with gofr package.
// Users should use gofr.RequireRole() instead for type safety.
type HandlerFunc func(ctx any) (any, error)

// RequireRole wraps a handler to require a specific role.
// Returns ErrAccessDenied if the user's role doesn't match.
// Note: For GoFr applications, use gofr.RequireRole() instead for better type safety.
func RequireRole(allowedRole string, handlerFunc HandlerFunc) HandlerFunc {
	return func(ctx any) (any, error) {
		// Type assert to get context value access
		type contextValueGetter interface {
			Value(key any) any
		}

		var role string

		if ctxWithValue, ok := ctx.(contextValueGetter); ok {
			if roleVal := ctxWithValue.Value(userRole); roleVal != nil {
				role, _ = roleVal.(string)
			}
		}

		if role == allowedRole {
			return handlerFunc(ctx)
		}

		return nil, ErrAccessDenied
	}
}

// RequireAnyRole wraps a handler to require any of the specified roles.
// Note: For GoFr applications, use gofr.RequireAnyRole() instead for better type safety.
func RequireAnyRole(allowedRoles []string, handlerFunc HandlerFunc) HandlerFunc {
	return func(ctx any) (any, error) {
		type contextValueGetter interface {
			Value(key any) any
		}

		var role string

		if ctxWithValue, ok := ctx.(contextValueGetter); ok {
			if roleVal := ctxWithValue.Value(userRole); roleVal != nil {
				role, _ = roleVal.(string)
			}
		}

		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				return handlerFunc(ctx)
			}
		}

		return nil, ErrAccessDenied
	}
}
