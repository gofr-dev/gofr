package rbac

import (
	"context"
	"errors"
	"net/http"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

type authMethod int

const userRole authMethod = 4

var (
	// ErrAccessDenied is returned when a user doesn't have required role/permission
	ErrAccessDenied = errors.New("forbidden: access denied")

	// ErrRoleNotFound is returned when role cannot be extracted from request
	ErrRoleNotFound = errors.New("unauthorized: role not found")
)

// auditLogger is an internal logger for authorization decisions.
// Audit logging is automatically performed using GoFr's logger - users don't need to configure this.
type auditLogger struct{}

// logAccess logs an authorization decision using GoFr's logger.
func (l *auditLogger) logAccess(logger logging.Logger, req *http.Request, role, route string, allowed bool, reason string) {
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
// The container is only passed to RoleExtractorFunc when config.RequiresContainer is true.
// This flag is automatically set by app.EnableRBAC*() methods:
//   - Header-based RBAC: RequiresContainer = false (container not passed)
//   - JWT-based RBAC: RequiresContainer = false (container not passed)
//   - Database-based RBAC: RequiresContainer = true (container passed)
//
// This ensures container access is restricted and only available when needed for security.
// Users should use app.EnableRBAC(), app.EnableRBACWithJWT(), app.EnableRBACWithPermissions(), etc.
// instead of calling this function directly.
func Middleware(config *Config, args ...any) func(handler http.Handler) http.Handler {
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
			var role string
			var err error

			if config.RoleExtractorFunc != nil {
				// Pass container only if explicitly required (database-based role extraction)
				// For header/JWT-based extraction, container is not needed and not passed
				extractorArgs := []any{}
				
				// Only pass container if RequiresContainer is true (database-based extraction)
				if config.RequiresContainer {
					if len(args) > 0 {
						if cntr, ok := args[0].(*container.Container); ok && cntr != nil {
							extractorArgs = append(extractorArgs, cntr)
						}
						// Append any additional args that were passed
						if len(args) > 1 {
							extractorArgs = append(extractorArgs, args[1:]...)
						}
					}
				} else {
					// For header/JWT-based extraction, only pass additional args (not container)
					if len(args) > 0 {
						// Skip first arg (container) if it's a container, otherwise include all args
						startIdx := 0
						if len(args) > 0 {
							if _, ok := args[0].(*container.Container); ok {
								startIdx = 1 // Skip container
							}
						}
						if startIdx < len(args) {
							extractorArgs = append(extractorArgs, args[startIdx:]...)
						}
					}
				}

				role, err = config.RoleExtractorFunc(r, extractorArgs...)
				if err != nil {
					// Use default role if configured
					if config.DefaultRole != "" {
						role = config.DefaultRole
					} else {
						handleAuthError(w, r, config, "", route, ErrRoleNotFound)
						return
					}
				}
			} else if config.DefaultRole != "" {
				role = config.DefaultRole
			} else {
				handleAuthError(w, r, config, "", route, ErrRoleNotFound)
				return
			}

			// Check authorization
			authorized := false
			authReason := ""

			// Check permission-based access if enabled
			if config.EnablePermissions && config.PermissionConfig != nil {
				// Store role in request context first for permission check
				reqCtx := context.WithValue(r.Context(), userRole, role)
				reqWithRole := r.WithContext(reqCtx)
				if err := CheckPermission(reqWithRole, config.PermissionConfig); err == nil {
					authorized = true
					authReason = "permission-based"
				}
			}

			// Check role-based access (if not already authorized by permissions)
			if !authorized {
				// Use hierarchy if configured
				if len(config.RoleHierarchy) > 0 {
					hierarchy := NewRoleHierarchy(config.RoleHierarchy)
					if IsRoleAllowedWithHierarchy(role, route, config, hierarchy) {
						authorized = true
						authReason = "role-based (with hierarchy)"
					}
				} else {
					if isRoleAllowed(role, route, config) {
						authorized = true
						authReason = "role-based"
					}
				}
			}

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

// logAuditEvent logs authorization decisions for audit purposes.
// This is called automatically by the middleware when Logger is set.
// Users don't need to configure this - it uses GoFr's logger automatically.
func logAuditEvent(logger logging.Logger, r *http.Request, role, route string, allowed bool, reason string) {
	auditLogger := &auditLogger{}
	auditLogger.logAccess(logger, r, role, route, allowed, reason)
}

// HandlerFunc is a function type that matches GoFr's handler signature.
// This avoids import cycle with gofr package.
// Users should use gofr.RequireRole() instead for type safety.
type HandlerFunc func(ctx interface{}) (any, error)

// RequireRole wraps a handler to require a specific role.
// Returns ErrAccessDenied if the user's role doesn't match.
// Note: For GoFr applications, use gofr.RequireRole() instead for better type safety.
func RequireRole(allowedRole string, handlerFunc HandlerFunc) HandlerFunc {
	return func(ctx interface{}) (any, error) {
		// Type assert to get context value access
		type contextValueGetter interface {
			Value(key interface{}) interface{}
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
	return func(ctx interface{}) (any, error) {
		type contextValueGetter interface {
			Value(key interface{}) interface{}
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
