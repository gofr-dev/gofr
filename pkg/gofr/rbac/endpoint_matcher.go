package rbac

import (
	"net/http"
	"regexp"
	"strings"
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

// matchesRegexPattern matches a route against a regex pattern using precompiled regex if available.
func matchesRegexPattern(pattern, route string, config *Config) bool {
	if config == nil {
		// Fallback to runtime compilation if config is not available
		matched, err := regexp.MatchString(pattern, route)
		return err == nil && matched
	}

	// Look up precompiled regex (stored with pattern as key during config processing)
	config.mu.RLock()
	compiled, exists := config.compiledRegexMap[pattern]
	config.mu.RUnlock()

	if exists {
		return compiled.MatchString(route)
	}

	// Compile and cache if not found (fallback for runtime compilation)
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return false // Invalid regex = no match
	}

	config.mu.Lock()
	config.compiledRegexMap[pattern] = compiled
	config.mu.Unlock()

	return compiled.MatchString(route)
}

// matchesEndpointPattern checks if the route matches the endpoint pattern.
// Method matching is handled separately in matchEndpoint before this function is called.
// Automatically detects if Path contains a regex pattern and uses appropriate matching.
func matchesEndpointPattern(endpoint *EndpointMapping, route string, config *Config) bool {
	if endpoint.Path == "" {
		return false
	}

	pattern := endpoint.Path

	// Check if pattern is a regex (starts with ^, ends with $, or contains regex special chars)
	if isRegexPattern(pattern) {
		return matchesRegexPattern(pattern, route, config)
	}

	// Check path pattern matching
	return matchesPathPattern(pattern, route)
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
func getEndpointForRequest(r *http.Request, config *Config) (*EndpointMapping, bool) {
	if len(config.Endpoints) == 0 {
		return nil, false
	}

	method := r.Method
	route := r.URL.Path

	return matchEndpoint(method, route, config.Endpoints, config)
}
