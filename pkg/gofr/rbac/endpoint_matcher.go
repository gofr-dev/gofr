package rbac

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

const (
	// DefaultConfigPath is the default config path value (empty string).
	// When passed to ResolveRBACConfigPath, it will try default paths: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
	DefaultConfigPath = ""

	// Default RBAC config paths (tried in order).
	defaultRBACJSONPath = "configs/rbac.json"
	defaultRBACYAMLPath = "configs/rbac.yaml"
	defaultRBACYMLPath  = "configs/rbac.yml"
)

var (
	// errUnbalancedBraces is returned when a mux pattern has unbalanced braces.
	errUnbalancedBraces = errors.New("unbalanced braces in pattern")

	// errInvalidPattern is returned when mux cannot compile a pattern.
	errInvalidPattern = errors.New("invalid mux pattern")
)

// matchEndpoint checks if the request matches an endpoint configuration.
// This is the primary authorization check using the unified Endpoints configuration.
// Returns the matched endpoint and whether it's public.
func matchEndpoint(method, route string, endpoints []EndpointMapping, config *Config) (*EndpointMapping, bool) {
	return resolveEndpoint(strings.ToUpper(method), route, buildEndpointRules(endpoints), config)
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

	// Let mux compile the pattern. A pattern mux cannot parse - an unterminated character
	// class, for example - would otherwise load cleanly and then never match any request,
	// leaving the endpoint it was meant to govern unguarded.
	if err := mux.NewRouter().NewRoute().Path(pattern).GetError(); err != nil {
		return fmt.Errorf("%w: %s: %w", errInvalidPattern, pattern, err)
	}

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

	return config.resolve(strings.ToUpper(r.Method), r.URL.Path)
}

// ResolveRBACConfigPath resolves the RBAC config file path.
// If configFile is empty, tries default paths in order: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
func ResolveRBACConfigPath(configFile string) string {
	// If custom path provided, use it
	if configFile != "" {
		return configFile
	}

	// Try default paths in order
	defaultPaths := []string{
		defaultRBACJSONPath,
		defaultRBACYAMLPath,
		defaultRBACYMLPath,
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
