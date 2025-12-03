package rbac

import (
	"net/http"
	"path"
	"regexp"
	"strings"
)

// matchEndpoint checks if the request matches an endpoint configuration.
// This is the primary authorization check using the unified Endpoints configuration.
// Returns the matched endpoint and whether it's public.
func matchEndpoint(method, route string, endpoints []EndpointMapping) (*EndpointMapping, bool) {
	for i := range endpoints {
		endpoint := &endpoints[i]

		// Check if endpoint is public
		if endpoint.Public {
			if matchesEndpointPattern(endpoint, route) {
				return endpoint, true
			}
			continue
		}

		// Check method match
		if !matchesHTTPMethod(method, endpoint.Methods) {
			continue
		}

		// Check route match
		if matchesEndpointPattern(endpoint, route) {
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

// matchesEndpointPattern checks if the route matches the endpoint pattern.
// Method matching is handled separately in matchEndpoint before this function is called.
func matchesEndpointPattern(endpoint *EndpointMapping, route string) bool {
	// Regex takes precedence
	if endpoint.Regex != "" {
		matched, err := regexp.MatchString(endpoint.Regex, route)
		if err == nil && matched {
			return true
		}
	}

	// Check path pattern
	if endpoint.Path != "" {
		// Use path.Match for pattern matching
		if matched, _ := path.Match(endpoint.Path, route); matched {
			return true
		}

		// Check prefix match for patterns ending with /*
		if strings.HasSuffix(endpoint.Path, "/*") {
			prefix := strings.TrimSuffix(endpoint.Path, "/*")
			if route == prefix || strings.HasPrefix(route, prefix+"/") {
				return true
			}
		}
	}

	return false
}

// checkEndpointAuthorization checks if the user's role is authorized for the endpoint.
// Pure permission-based: only checks if role has the required permission.
// Uses the endpoint parameter directly instead of re-looking it up.
func checkEndpointAuthorization(role string, endpoint *EndpointMapping, config *Config) (bool, string) {
	// Public endpoints are always allowed
	if endpoint.Public {
		return true, "public-endpoint"
	}

	// Get required permission from the endpoint directly
	requiredPermission := endpoint.RequiredPermission

	// If no permission requirement found, deny (fail secure)
	if requiredPermission == "" {
		return false, ""
	}

	// Get role's permissions (thread-safe)
	rolePerms := config.GetRolePermissions(role)
	if len(rolePerms) == 0 {
		return false, ""
	}

	// Check if role has the required permission
	for _, perm := range rolePerms {
		// Exact match
		if perm == requiredPermission || perm == "*:*" {
			return true, "permission-based"
		}

		// Wildcard permission match (e.g., "users:*" matches "users:read")
		if strings.HasSuffix(perm, ":*") {
			permPrefix := strings.TrimSuffix(perm, ":*")
			if strings.HasPrefix(requiredPermission, permPrefix+":") {
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

	return matchEndpoint(method, route, config.Endpoints)
}
