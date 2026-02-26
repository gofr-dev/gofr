package rbac

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/http/middleware"
)

type authMethod int

const userRole authMethod = 4

const unknownRouteLabel = "<unmatched>"

// AuditLog represents a structured log entry for RBAC authorization decisions.
// It follows the same pattern as HTTP RequestLog for consistency.
type AuditLog struct {
	CorrelationID string `json:"correlation_id,omitempty"`
	Method        string `json:"method,omitempty"`
	Route         string `json:"route,omitempty"`
	Status        string `json:"status,omitempty"`
	Role          string `json:"role,omitempty"`
}

// PrettyPrint formats the RBAC audit log for terminal output, matching HTTP log format.
func (ral *AuditLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s %-6s %10s %s %s [%s]\u001B[0m\n",
		ral.CorrelationID, ral.Status, "", "RBAC", ral.Route, ral.Role)
}

var (
	// ErrAccessDenied is returned when a user doesn't have required role/permission.
	ErrAccessDenied = errors.New("forbidden: access denied")

	// ErrRoleNotFound is returned when role cannot be extracted from request.
	ErrRoleNotFound = errors.New("unauthorized: role not found")

	// errJWTClaimsNotFound is returned when JWT claims are not found in request context.
	errJWTClaimsNotFound = errors.New("JWT claims not found in request context")

	// errEmptyClaimPath is returned when claim path is empty.
	errEmptyClaimPath = errors.New("empty claim path")

	// errClaimPathNotFound is returned when a claim path is not found in JWT claims.
	errClaimPathNotFound = errors.New("claim path not found")

	// errInvalidArrayNotation is returned when array notation is invalid.
	errInvalidArrayNotation = errors.New("invalid array notation")

	// errInvalidArrayIndex is returned when array index is invalid.
	errInvalidArrayIndex = errors.New("invalid array index")

	// errClaimKeyNotFound is returned when a claim key is not found.
	errClaimKeyNotFound = errors.New("claim key not found")

	// errClaimValueNotArray is returned when a claim value is not an array.
	errClaimValueNotArray = errors.New("claim value is not an array")

	// errArrayIndexOutOfBounds is returned when array index is out of bounds.
	errArrayIndexOutOfBounds = errors.New("array index out of bounds")

	// errInvalidClaimStructure is returned when claim structure is invalid.
	errInvalidClaimStructure = errors.New("invalid claim structure")

	// errAuthorizationError is returned as a generic error message for unknown errors in traces.
	errAuthorizationError = errors.New("authorization error")
)

// Middleware creates an HTTP middleware function that enforces RBAC authorization.
// It extracts the user's role and checks if the role is allowed for the requested route.
//
//nolint:gocognit,gocyclo // Middleware complexity is acceptable due to multiple authorization paths
func Middleware(config *Config) func(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If config is nil, allow all requests (fail open)
			if config == nil {
				handler.ServeHTTP(w, r)

				return
			}

			// Check if endpoint is public using unified Endpoints config
			endpoint, isPublic := getEndpointForRequest(r, config)

			routeLabel := unknownRouteLabel
			if endpoint != nil && endpoint.Path != "" {
				routeLabel = endpoint.Path
			}

			// Start tracing if tracer is available
			if config.Tracer != nil {
				ctx, span := config.Tracer.Start(r.Context(), "rbac.authorize")
				span.SetAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.route", routeLabel),
				)
				r = r.WithContext(ctx)

				defer span.End()
			}

			if isPublic {
				handler.ServeHTTP(w, r)

				return
			}

			// If no endpoint match found in RBAC config, allow request to proceed to route matching
			// RBAC only enforces authorization for endpoints that are explicitly configured
			// Routes not in RBAC config are handled by normal route matching (may return 404 if route doesn't exist)
			if endpoint == nil {
				handler.ServeHTTP(w, r)

				return
			}

			// Extract role using header-based or JWT-based extraction
			role, err := extractRole(r, config)
			if err != nil {
				if config.Metrics != nil {
					config.Metrics.IncrementCounter(r.Context(), "rbac_role_extraction_failures")
				}

				handleAuthError(w, r, config, "", routeLabel, err)

				return
			}

			// Role not included in traces for privacy (roles are PII)
			// Only include authorization status (safe boolean) - set below after authorization check

			// Check authorization using unified endpoint-based authorization
			authorized, _ := checkEndpointAuthorization(role, endpoint, config)
			if !authorized {
				handleAuthError(w, r, config, role, routeLabel, ErrAccessDenied)

				return
			}

			if config.Tracer != nil {
				trace.SpanFromContext(r.Context()).SetAttributes(attribute.Bool("rbac.authorized", true))
			}

			if config.Logger != nil {
				logAuditEvent(config.Logger, r, role, routeLabel, true)
			}

			// Store role in context and continue
			ctx := context.WithValue(r.Context(), userRole, role)
			handler.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// handleAuthError handles authorization errors with custom error handler or default response.
func handleAuthError(w http.ResponseWriter, r *http.Request, config *Config, role, route string, err error) {
	// Record error in span if tracing is enabled
	// Sanitize error message to prevent information leakage
	if config.Tracer != nil {
		span := trace.SpanFromContext(r.Context())
		safeErr := sanitizeErrorForTrace(err)
		span.RecordError(safeErr)
		span.SetStatus(codes.Error, safeErr.Error())
	}

	// Log audit event (always enabled when Logger is available)
	// Audit logging is automatically performed using GoFr's logger
	if config.Logger != nil {
		logAuditEvent(config.Logger, r, role, route, false)
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
		return "", fmt.Errorf("%w", errJWTClaimsNotFound)
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
		return nil, fmt.Errorf("%w", errEmptyClaimPath)
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
		return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, path)
	}

	return value, nil
}

// extractArrayClaim extracts a value from an array in JWT claims.
func extractArrayClaim(claims jwt.MapClaims, path string, idx int) (any, error) {
	key := path[:idx]
	arrayPath := path[idx:]

	// Extract array index
	if !strings.HasPrefix(arrayPath, "[") || !strings.HasSuffix(arrayPath, "]") {
		return nil, fmt.Errorf("%w: %s", errInvalidArrayNotation, path)
	}

	indexStr := strings.Trim(arrayPath, "[]")

	var index int
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidArrayIndex, indexStr)
	}

	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errClaimKeyNotFound, key)
	}

	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errClaimValueNotArray, key)
	}

	if index < 0 || index >= len(arr) {
		return nil, fmt.Errorf("%w: %d (length: %d)", errArrayIndexOutOfBounds, index, len(arr))
	}

	return arr[index], nil
}

// extractNestedClaim extracts a value from nested structure in JWT claims.
func extractNestedClaim(claims jwt.MapClaims, path string) (any, error) {
	parts := strings.Split(path, ".")

	var current any = claims

	for i, part := range parts {
		isLast := i == len(parts)-1

		// Navigate through nested structure
		var next any

		var exists bool

		switch v := current.(type) {
		case map[string]any:
			next, exists = v[part]
		case jwt.MapClaims:
			next, exists = v[part]
		default:
			if isLast {
				return nil, fmt.Errorf("%w: %s", errInvalidClaimStructure, strings.Join(parts[:i+1], "."))
			}

			return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, strings.Join(parts[:i+1], "."))
		}

		if !exists {
			return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, strings.Join(parts[:i+1], "."))
		}

		if isLast {
			return next, nil // Return nil value if key exists but value is nil
		}

		// For intermediate paths, nil means invalid structure
		if next == nil {
			return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, strings.Join(parts[:i+1], "."))
		}

		current = next
	}

	return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, path)
}

// logAuditEvent logs authorization decisions for audit purposes.
// This is called automatically by the middleware when Logger is set.
// Users don't need to configure this - it uses the provided logger automatically.
func logAuditEvent(logger datasource.Logger, r *http.Request, role, route string, allowed bool) {
	if logger == nil {
		return // Skip logging if no logger provided
	}

	status := "REJ"
	if allowed {
		status = "ACC"
	}

	// Extract correlation ID from trace context
	correlationID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
	if correlationID == "" || correlationID == "00000000000000000000000000000000" {
		correlationID = "<no-trace>"
	}

	// Create structured audit log entry
	auditLog := &AuditLog{
		CorrelationID: correlationID,
		Method:        r.Method,
		Route:         route,
		Status:        status,
		Role:          role,
	}

	// Use structured logging at debug level (logger will handle JSON encoding or PrettyPrint)
	logger.Debug(auditLog)
}

// sanitizeErrorForTrace sanitizes error messages for traces to prevent information leakage.
// Returns generic error messages that don't expose internal system details.
func sanitizeErrorForTrace(err error) error {
	if errors.Is(err, ErrRoleNotFound) {
		return ErrRoleNotFound // Safe: generic error message
	}

	if errors.Is(err, ErrAccessDenied) {
		return ErrAccessDenied // Safe: generic error message
	}

	// For unknown errors, return generic message to prevent information leakage
	return errAuthorizationError
}
