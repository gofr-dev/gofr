package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/http/middleware"
)

type authMethod int

const userRole authMethod = 4

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
			r = startTracing(r, config, route)

			// End span at the end of the middleware function (covers all code paths)
			// If tracing was started, the span will be in the context
			if config.Tracer != nil {
				span := trace.SpanFromContext(r.Context())
				if span != nil {
					defer span.End()
				}
			}

			// Check if endpoint is public using unified Endpoints config
			endpoint, isPublic := getEndpointForRequest(r, config)
			if isPublic {
				handler.ServeHTTP(w, r)

				return
			}

			// If no endpoint match found, deny by default (fail secure)
			if endpoint == nil {
				recordMetrics(config, r, "denied", "endpoint_not_found", "")
				handleAuthError(w, r, config, "", route, ErrAccessDenied)

				return
			}

			// Extract role using header-based or JWT-based extraction
			role, err := extractRole(r, config)
			if err != nil {
				recordRoleExtractionFailure(config, r)
				handleAuthError(w, r, config, "", route, err)

				return
			}

			// Update span with role
			updateSpanWithRole(config, r, role)

			// Check authorization using unified endpoint-based authorization
			authorized, authReason := checkEndpointAuthorization(role, endpoint, config)
			if !authorized {
				recordMetrics(config, r, "denied", "", role)
				handleAuthError(w, r, config, role, route, ErrAccessDenied)

				return
			}

			recordMetrics(config, r, "allowed", "", role)
			updateSpanWithAuthStatus(config, r)
			logAuditEventIfEnabled(config, r, role, route, authReason)

			// Store role in context and continue
			ctx := context.WithValue(r.Context(), userRole, role)
			handler.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// startTracing starts tracing for the request if tracer is available.
// The span is stored in the context and should be ended at the end of the middleware function.
func startTracing(r *http.Request, config *Config, route string) *http.Request {
	if config.Tracer == nil {
		return r
	}

	ctx, span := config.Tracer.Start(r.Context(), "rbac.authorize")

	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", route),
	)

	return r.WithContext(ctx)
}

// recordMetrics records authorization decision metrics.
func recordMetrics(config *Config, r *http.Request, status, reason, role string) {
	if config.Metrics == nil {
		return
	}

	labels := []string{"status", status}
	if reason != "" {
		labels = append(labels, "reason", reason)
	}

	if role != "" {
		labels = append(labels, "role", role)
	}

	config.Metrics.IncrementCounter(r.Context(), "rbac_authorization_decisions", labels...)
}

// recordRoleExtractionFailure records role extraction failure metrics.
func recordRoleExtractionFailure(config *Config, r *http.Request) {
	if config.Metrics != nil {
		config.Metrics.IncrementCounter(r.Context(), "rbac_role_extraction_failures")
	}
}

// updateSpanWithRole updates the span with the extracted role.
func updateSpanWithRole(config *Config, r *http.Request, role string) {
	if config.Tracer != nil {
		trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("rbac.role", role))
	}
}

// updateSpanWithAuthStatus updates the span with authorization status.
func updateSpanWithAuthStatus(config *Config, r *http.Request) {
	if config.Tracer != nil {
		trace.SpanFromContext(r.Context()).SetAttributes(attribute.Bool("rbac.authorized", true))
	}
}

// logAuditEventIfEnabled logs audit event if logger is available.
func logAuditEventIfEnabled(config *Config, r *http.Request, role, route, authReason string) {
	if config.Logger != nil {
		logAuditEvent(config.Logger, r, role, route, true, authReason)
	}
}

// handleAuthError handles authorization errors with custom error handler or default response.
func handleAuthError(w http.ResponseWriter, r *http.Request, config *Config, role, route string, err error) {
	// Record error in span if tracing is enabled
	if config.Tracer != nil {
		span := trace.SpanFromContext(r.Context())
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

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
		if isLast {
			return extractFinalClaimValue(current, part, path, parts, i)
		}

		// Navigate through nested structure
		current = navigateNestedClaim(current, part)
		if current == nil {
			return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, strings.Join(parts[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, path)
}

// extractFinalClaimValue extracts the final value from a claim path.
func extractFinalClaimValue(current any, part, path string, parts []string, i int) (any, error) {
	if m, ok := current.(map[string]any); ok {
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, path)
		}

		return value, nil
	}

	if m, ok := current.(jwt.MapClaims); ok {
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("%w: %s", errClaimPathNotFound, path)
		}

		return value, nil
	}

	return nil, fmt.Errorf("%w: %s", errInvalidClaimStructure, strings.Join(parts[:i+1], "."))
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
