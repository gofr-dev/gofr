package rbac

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/datasource"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/internal/routemark"
)

type authMethod int

const userRole authMethod = 4

const unknownRouteLabel = "<unmatched>"

// matchedRouteLabel returns the route template the router matched for r, which is what http.route
// means everywhere else in a trace. The RBAC rule that governed the request can be broader or
// narrower than that route, so it is recorded separately as rbac.rule.
//
// GoFr's catch-all, which serves every request no route matched, is reported as unmatched. An
// application route or a static directory registered at "/" shares its template, so the two are
// told apart by the mark GoFr puts on the catch-all when it registers it.
func matchedRouteLabel(r *http.Request) string {
	tmpl := gofrHTTP.RouteTemplate(r)
	if tmpl == "" || routemark.CatchAllAnswered(r) {
		return unknownRouteLabel
	}

	return tmpl
}

// wellKnownPrefix is the path prefix the OAuth middleware passes through without verifying a token.
const wellKnownPrefix = "/.well-known"

// AuditLog represents a structured log entry for RBAC authorization decisions.
// It follows the same pattern as HTTP RequestLog for consistency.
type AuditLog struct {
	CorrelationID string `json:"correlation_id,omitempty"`
	Method        string `json:"method,omitempty"`
	Route         string `json:"route,omitempty"`
	Status        string `json:"status,omitempty"`
	// Role is the role that granted access. On a denial it is the role only when a single role was
	// held; with a role array or permissions claim it is empty and HeldCount says how many were held
	// instead.
	Role string `json:"role,omitempty"`
	// Permission is, in permissions mode, the permission that granted access.
	Permission string `json:"permission,omitempty"`
	HeldCount  int    `json:"held_count"`
}

// PrettyPrint formats the RBAC audit log for terminal output, matching HTTP log format.
func (ral *AuditLog) PrettyPrint(writer io.Writer) {
	held := ral.Role
	if held == "" {
		held = ral.Permission
	}

	if held == "" && ral.HeldCount > 0 {
		held = fmt.Sprintf("%d held", ral.HeldCount)
	}

	fmt.Fprintf(writer, "\u001B[38;5;8m%s %-6s %10s %s %s [%s]\u001B[0m\n",
		ral.CorrelationID, ral.Status, "", "RBAC", ral.Route, held)
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
	var warnings extractionWarnings

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
					attribute.String("http.route", matchedRouteLabel(r)),
					attribute.String("rbac.rule", routeLabel),
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

			// Extract what the request holds, using header-based or JWT-based extraction
			h, err := extractHeld(r, config)
			if err != nil {
				reportExtractionFailure(r, config, err, &warnings)
				handleAuthError(w, r, config, &denial{route: routeLabel, err: bareError(err)})

				return
			}

			// Role not included in traces for privacy (roles are PII)
			// Only include authorization status (safe boolean) - set below after authorization check

			// Check authorization using unified endpoint-based authorization
			authorized, granted := checkEndpointAuthorization(h, endpoint, config)
			if !authorized {
				// A single role is reported as before; several are reported only as a count.
				if h.multi() {
					handleAuthError(w, r, config, &denial{route: routeLabel, heldCount: h.count(), err: ErrAccessDenied})
				} else {
					handleAuthError(w, r, config, &denial{role: h.str, route: routeLabel, err: ErrAccessDenied})
				}

				return
			}

			if config.Tracer != nil {
				trace.SpanFromContext(r.Context()).SetAttributes(attribute.Bool("rbac.authorized", true))
			}

			if config.Logger != nil {
				entry := &AuditLog{Route: routeLabel, Role: granted}
				if h.permissionsMode {
					entry.Role, entry.Permission = "", granted
				}

				logAuditEvent(config.Logger, r, entry, true)
			}

			// Store what granted access in context and continue: the granting role, or in permissions
			// mode the granting permission
			ctx := context.WithValue(r.Context(), userRole, granted)
			handler.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractionWarnings holds the one-shot guards for the warnings reportExtractionFailure can raise, so
// each is logged once per Config however many requests hit it.
type extractionWarnings struct {
	callOrder        sync.Once
	unreadable       sync.Once
	extractorTimeout sync.Once
}

// extractionFailure is why a role or permissions could not be read. Its Error is the bare sentinel
// (ErrRoleNotFound or ErrAccessDenied) a custom ErrorHandler has always received; reason and cause
// feed the metric label and the logs and never reach the client or the handler. In extractor mode the
// sentinel is the extractor's own error, or a 503 error wrapping the context's on a timeout.
type extractionFailure struct {
	sentinel error  // ErrRoleNotFound (401), ErrAccessDenied (403), or the extractor's error
	cause    error  // what went wrong, for logs; may be nil
	reason   string // the reason label on the extraction failures metric
}

func (f *extractionFailure) Error() string { return f.sentinel.Error() }

// bareError returns the sentinel inside an extraction failure, or err itself when it is not one.
func bareError(err error) error {
	if f, ok := errors.AsType[*extractionFailure](err); ok {
		return f.sentinel
	}

	return err
}

// reportExtractionFailure counts a request whose role or permissions could not be read, labeled by
// cause, and warns about the two causes an operator can fix: a claim of an unreadable type (named by
// its path, never its value), and RBAC running without the OAuth middleware in front of it. Each warning
// is logged once per Config; the metric still counts every request.
func reportExtractionFailure(r *http.Request, config *Config, err error, warnings *extractionWarnings) {
	f, ok := errors.AsType[*extractionFailure](err)
	if !ok {
		return
	}

	if config.Metrics != nil {
		config.Metrics.IncrementCounter(r.Context(), extractionFailuresMetric, "reason", f.reason)
	}

	if config.Logger == nil {
		return
	}

	switch {
	case f.reason == reasonExtractorTimeout && errors.Is(f.cause, context.DeadlineExceeded):
		// A canceled request is the client going away, not a slow extractor.
		warnings.extractorTimeout.Do(func() {
			config.Logger.Warnf("RBAC: extractor did not return within %s; request denied with 503", config.extractorTimeout)
		})
	case f.reason == reasonUnreadableClaim:
		warnings.unreadable.Do(func() {
			_, path := config.mode()
			config.Logger.Warnf("RBAC: claim %q is not a string or an array of strings; request denied", path)
		})
	case f.reason == reasonNoJWTClaims && !isWellKnown(r.URL.Path):
		// The OAuth middleware never verifies /.well-known/*, so no claims there is not a call-order
		// problem.
		warnings.callOrder.Do(func() {
			config.Logger.Warnf("RBAC: no JWT claims in the request; call EnableOAuth before EnableRBAC")
		})
	}
}

// Values of the reason label on the extraction failures metric.
const (
	reasonNoJWTClaims     = "no_jwt_claims"
	reasonUnreadableClaim = "unreadable_claim"
	reasonMissingRole     = "missing_role"
)

// isWellKnown reports whether path is under /.well-known, which the OAuth middleware does not verify.
func isWellKnown(path string) bool {
	return path == wellKnownPrefix || strings.HasPrefix(path, wellKnownPrefix+"/")
}

// denial is what a denied request is reported with: the audit log entry, and the error a custom
// ErrorHandler receives.
type denial struct {
	role      string // the single role held, when there was exactly one
	route     string
	heldCount int // how many roles or permissions were held, when more than one could be
	err       error
}

// handleAuthError handles authorization errors with custom error handler or default response.
func handleAuthError(w http.ResponseWriter, r *http.Request, config *Config, d *denial) {
	// Record error in span if tracing is enabled
	// Sanitize error message to prevent information leakage
	if config.Tracer != nil {
		span := trace.SpanFromContext(r.Context())
		safeErr := sanitizeErrorForTrace(d.err)
		span.RecordError(safeErr)
		span.SetStatus(codes.Error, safeErr.Error())
	}

	// Log audit event (always enabled when Logger is available)
	// Audit logging is automatically performed using GoFr's logger
	if config.Logger != nil {
		logAuditEvent(config.Logger, r, &AuditLog{Route: d.route, Role: d.role, HeldCount: d.heldCount}, false)
	}

	// Use custom error handler if provided
	if config.ErrorHandler != nil {
		config.ErrorHandler(w, r, d.role, d.route, d.err)
		return
	}

	// An extractor's error can choose its status, if that status is an error. Only the status text is
	// sent, so the error's own message never reaches the client.
	var sc gofrHTTP.StatusCodeResponder
	if errors.As(d.err, &sc) && sc.StatusCode() >= http.StatusBadRequest {
		code := sc.StatusCode()
		http.Error(w, http.StatusText(code), code)

		return
	}

	// Default error handling. A 401 means no usable credential was presented; a 403 means it was, but
	// holds nothing that grants access. Only the status codes follow RFC 6750 §3.1; no WWW-Authenticate
	// header is set.
	if errors.Is(d.err, ErrRoleNotFound) {
		http.Error(w, "Unauthorized: Missing or invalid role", http.StatusUnauthorized)

		return
	}

	http.Error(w, "Forbidden: Access denied", http.StatusForbidden)
}

// extractHeld extracts what the request holds for authorization.
// Supports the application's extractor (via WithExtractor), permissions-mode extraction (via
// PermissionsClaimPath), role extraction from JWT claims (via JWTClaimPath), or header-based role
// extraction (via RoleHeader), in that order of precedence.
// When a JWT claim path is set the header is never consulted, even if JWT extraction fails.
// No default role is supported - role must be explicitly provided.
// A failure is an *extractionFailure.
func extractHeld(r *http.Request, config *Config) (held, error) {
	switch mode, _ := config.mode(); mode {
	case modeExtractor:
		return extractHeldFromExtractor(r, config)
	case modePermissions, modeRoles:
		return extractHeldFromJWT(r, config)
	case modeHeader:
		if role := r.Header.Get(config.RoleHeader); role != "" {
			return held{str: role}, nil
		}
	}

	// No role found - no default role supported
	return held{}, &extractionFailure{sentinel: ErrRoleNotFound, reason: reasonMissingRole}
}

// extractHeldFromJWT reads the claim named by the configured mode from the JWT claims in the request
// context. It trusts the OAuth middleware to have validated the token, including aud and exp.
func extractHeldFromJWT(r *http.Request, config *Config) (held, error) {
	mode, claimPath := config.mode()
	permissionsMode := mode == modePermissions

	// Get JWT claims from context (set by OAuth middleware)
	claims, ok := r.Context().Value(middleware.JWTClaim).(jwt.MapClaims)
	if !ok || claims == nil {
		return held{}, &extractionFailure{sentinel: ErrRoleNotFound, cause: errJWTClaimsNotFound, reason: reasonNoJWTClaims}
	}

	value, err := extractClaimValue(claims, claimPath)
	if err != nil {
		// A verified token without a permissions claim holds no permissions: a 403, not a 401.
		if permissionsMode {
			return held{permissionsMode: true}, nil
		}

		// Do not fall back to the header: JWT is the only method when configured
		return held{}, &extractionFailure{sentinel: ErrRoleNotFound, cause: err, reason: reasonMissingRole}
	}

	h, err := heldFromClaim(value, permissionsMode)
	if err == nil {
		return h, nil
	}

	if errors.Is(err, errUnreadableClaim) {
		return held{}, &extractionFailure{sentinel: ErrAccessDenied, cause: err, reason: reasonUnreadableClaim}
	}

	return held{}, &extractionFailure{sentinel: ErrRoleNotFound, cause: err, reason: reasonMissingRole}
}

// extractClaimValue extracts a value from JWT claims by reading path as a dot-notation or array
// notation path. Only when that finds nothing is the whole path tried as one top-level key, so a
// namespaced claim name that contains dots still works without changing what an existing path reads.
// Examples:
//   - "role" -> claims["role"]
//   - "https://example.com/roles" -> claims["https://example.com/roles"]
//   - "roles[0]" -> claims["roles"].([]any)[0]
//   - "permissions.role" -> claims["permissions"].(map[string]any)["role"]
func extractClaimValue(claims jwt.MapClaims, path string) (any, error) {
	if path == "" {
		return nil, fmt.Errorf("%w", errEmptyClaimPath)
	}

	value, err := walkClaimPath(claims, path)
	if err == nil {
		return value, nil
	}

	if value, ok := claims[path]; ok {
		return value, nil
	}

	return nil, err
}

// walkClaimPath reads path as array notation ("roles[0]"), dot notation ("permissions.role"), or a
// plain key.
func walkClaimPath(claims jwt.MapClaims, path string) (any, error) {
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

// parseArrayPath splits "roles[0]" into its key and index.
func parseArrayPath(path string, idx int) (key string, index int, err error) {
	arrayPath := path[idx:]

	if !strings.HasPrefix(arrayPath, "[") || !strings.HasSuffix(arrayPath, "]") {
		return "", 0, fmt.Errorf("%w: %s", errInvalidArrayNotation, path)
	}

	indexStr := strings.Trim(arrayPath, "[]")

	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		return "", 0, fmt.Errorf("%w: %s", errInvalidArrayIndex, indexStr)
	}

	return path[:idx], index, nil
}

// validateClaimPath reports whether path can be read at all: an array notation path must parse.
func validateClaimPath(path string) error {
	if idx := strings.Index(path, "["); idx != -1 {
		_, _, err := parseArrayPath(path, idx)

		return err
	}

	return nil
}

// extractArrayClaim extracts a value from an array in JWT claims.
func extractArrayClaim(claims jwt.MapClaims, path string, idx int) (any, error) {
	key, index, err := parseArrayPath(path, idx)
	if err != nil {
		return nil, err
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
// entry carries the route and what was held; the correlation ID, method and status are filled in here.
func logAuditEvent(logger datasource.Logger, r *http.Request, entry *AuditLog, allowed bool) {
	if logger == nil {
		return // Skip logging if no logger provided
	}

	entry.Status = "REJ"
	if allowed {
		entry.Status = "ACC"
	}

	// Extract correlation ID from trace context
	entry.CorrelationID = trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
	if entry.CorrelationID == "" || entry.CorrelationID == "00000000000000000000000000000000" {
		entry.CorrelationID = "<no-trace>"
	}

	entry.Method = r.Method

	// Use structured logging at debug level (logger will handle JSON encoding or PrettyPrint)
	logger.Debug(entry)
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
