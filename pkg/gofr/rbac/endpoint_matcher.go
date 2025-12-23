package rbac

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
)

var (
	// errUnbalancedBraces is returned when a mux pattern has unbalanced braces.
	errUnbalancedBraces = errors.New("unbalanced braces in pattern")
)

// matchEndpoint checks if the request matches an endpoint configuration.
// This is the primary authorization check using the unified Endpoints configuration.
// Returns the matched endpoint and whether it's public.
func matchEndpoint(method, route string, endpoints []EndpointMapping, config *Config) (*EndpointMapping, bool) {
	for i := range endpoints {
		endpoint := &endpoints[i]

		// Check if endpoint is public
		if endpoint.Public {
			if matchesEndpointPattern(endpoint, route, config) {
				return endpoint, true
			}

			continue
		}

		// Check method match
		if !matchesHTTPMethod(method, endpoint.Methods) {
			continue
		}

		// Check route match
		if matchesEndpointPattern(endpoint, route, config) {
			return endpoint, false
		}
	}

	return nil, false
}

// matchesHTTPMethod checks if the HTTP method matches the endpoint's allowed methods.
func matchesHTTPMethod(method string, allowedMethods []string) bool {
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

// isMuxPattern detects if a pattern contains mux-style variables.
// Returns true if pattern contains { and }.
func isMuxPattern(pattern string) bool {
	return strings.Contains(pattern, "{") && strings.Contains(pattern, "}")
}

// matchMuxPattern uses mux Route.Match() to test if a path matches a mux pattern.
// Creates a temporary mux Route and uses Route.Match() to test the pattern.
// Handles all mux pattern types: {id}, {id:[0-9]+}, {path:.*}, etc.
func matchMuxPattern(pattern, method, path string, router *mux.Router) bool {
	if router == nil {
		return false
	}

	// Create a temporary route with the pattern
	route := router.NewRoute().Path(pattern)

	// If method is specified, add it to the route
	if method != "" {
		route = route.Methods(method)
	}

	// Create a mock request for matching
	req := &http.Request{
		Method: method,
		URL: &url.URL{
			Path: path,
		},
	}

	// Use Route.Match() to test if the request matches the pattern
	var match mux.RouteMatch

	return route.Match(req, &match)
}

// validateMuxPattern validates mux pattern syntax.
// Ensures balanced braces and validates regex constraints format.
func validateMuxPattern(pattern string) error {
	// Check for balanced braces
	openCount := strings.Count(pattern, "{")

	closeCount := strings.Count(pattern, "}")

	if openCount != closeCount {
		return fmt.Errorf("%w: %s", errUnbalancedBraces, pattern)
	}

	// Check that if there are closing braces, there must be opening braces
	// A pattern like "/api/id}" should not be valid
	if closeCount > 0 && openCount == 0 {
		return fmt.Errorf("%w: %s", errUnbalancedBraces, pattern)
	}

	// Basic validation: check that braces are properly formatted
	// More detailed validation would require parsing, which mux will do anyway
	return nil
}

// matchesEndpointPattern checks if the route matches the endpoint pattern.
// Method matching is handled separately in matchEndpoint before this function is called.
// Uses mux Route.Match() for mux patterns, exact match for non-pattern paths.
func matchesEndpointPattern(endpoint *EndpointMapping, route string, config *Config) bool {
	if endpoint.Path == "" {
		return false
	}

	pattern := endpoint.Path

	// Exact match for non-pattern paths
	if !isMuxPattern(pattern) {
		return pattern == route
	}

	// Use mux Route.Match() for patterns
	// Method is handled separately, so pass empty string here
	return matchMuxPattern(pattern, "", route, config.muxRouter)
}

// checkEndpointAuthorization checks if the user's role is authorized for the endpoint.
// Pure permission-based: checks if role has ANY of the required permissions (OR logic).
// Uses the endpoint parameter directly instead of re-looking it up.
func checkEndpointAuthorization(role string, endpoint *EndpointMapping, config *Config) (allowed bool, reason string) {
	// Public endpoints are always allowed
	if endpoint.Public {
		return true, "public-endpoint"
	}

	// Get required permissions
	requiredPerms := endpoint.RequiredPermissions

	// If no permission requirement found, deny (fail secure)
	if len(requiredPerms) == 0 {
		return false, ""
	}

	// Get role's permissions (thread-safe)
	rolePerms := config.GetRolePermissions(role)
	if len(rolePerms) == 0 {
		return false, ""
	}

	// Check if role has ANY of the required permissions (OR logic)
	// Only exact matches are supported - wildcards are NOT supported in permissions
	for _, requiredPerm := range requiredPerms {
		for _, perm := range rolePerms {
			// Exact match only - no wildcard support
			if perm == requiredPerm {
				return true, "permission-based"
			}
		}
	}

	return false, ""
}

// getEndpointForRequest finds the matching endpoint configuration for a request.
// This is the primary function used by the middleware to determine authorization requirements.
// Uses optimized maps for O(1) exact matches, falls back to pattern matching for mux patterns.
func getEndpointForRequest(r *http.Request, config *Config) (*EndpointMapping, bool) {
	if len(config.Endpoints) == 0 {
		return nil, false
	}

	method := strings.ToUpper(r.Method)
	path := r.URL.Path
	key := fmt.Sprintf("%s:%s", method, path)

	// Try exact match first (O(1) lookup)
	if endpoint, isPublic := config.getExactEndpoint(key); endpoint != nil {
		return endpoint, isPublic
	}

	// Try pattern matching (O(n) but only for patterns, not exact matches)
	return config.findEndpointByPattern(method, path)
}
