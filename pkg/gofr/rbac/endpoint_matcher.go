package rbac

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
			if matchesEndpointPattern(endpoint, method, route) {
				return endpoint, true
			}
			continue
		}

		// Check method match
		if !matchesHTTPMethod(method, endpoint.Methods) {
			continue
		}

		// Check route match
		if matchesEndpointPattern(endpoint, method, route) {
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
func matchesEndpointPattern(endpoint *EndpointMapping, method, route string) bool {
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
// This follows industry best practices:
// 1. Permission-based check (if requiredPermission is set)
// 2. Role-based check (if allowedRoles is set)
// 3. Attribute-based check (if role has attributes)
func checkEndpointAuthorization(r *http.Request, role string, endpoint *EndpointMapping, config *Config) (bool, string) {
	// If both RequiredPermission and AllowedRoles are set, both must pass (AND logic)
	permissionCheck := true
	roleCheck := true

	// Check permission-based authorization
	if endpoint.RequiredPermission != "" {
		// Get role's permissions from unified config
		rolePerms, exists := config.rolePermissionsMap[role]
		if !exists {
			permissionCheck = false
		} else {
			// Check if role has the required permission
			hasPermission := false
			for _, perm := range rolePerms {
				if perm == endpoint.RequiredPermission || perm == "*:*" {
					hasPermission = true
					break
				}
				// Support wildcard permissions (e.g., "users:*" matches "users:read")
				if strings.HasSuffix(perm, ":*") {
					permPrefix := strings.TrimSuffix(perm, ":*")
					if strings.HasPrefix(endpoint.RequiredPermission, permPrefix+":") {
						hasPermission = true
						break
					}
				}
			}
			permissionCheck = hasPermission
		}
	}

	// Check role-based authorization
	if len(endpoint.AllowedRoles) > 0 {
		hasRole := false
		for _, allowedRole := range endpoint.AllowedRoles {
			if allowedRole == role || allowedRole == "*" {
				hasRole = true
				break
			}
		}

		// Also check role hierarchy from InheritsFrom in Roles
		if !hasRole {
			// Check if role inherits any of the allowed roles
			effectiveRoles := getEffectiveRolesFromConfig(role, config)
			for _, effectiveRole := range effectiveRoles {
				for _, allowedRole := range endpoint.AllowedRoles {
					if effectiveRole == allowedRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}
		}

		roleCheck = hasRole
	}

	// Both checks must pass if both are configured
	if endpoint.RequiredPermission != "" && len(endpoint.AllowedRoles) > 0 {
		if permissionCheck && roleCheck {
			return true, "permission-and-role-based"
		}
		return false, ""
	}

	// If only permission check is configured
	if endpoint.RequiredPermission != "" {
		if permissionCheck {
			return true, "permission-based"
		}
		return false, ""
	}

	// If only role check is configured
	if len(endpoint.AllowedRoles) > 0 {
		if roleCheck {
			return true, "role-based"
		}
		return false, ""
	}

	// If neither is configured, deny by default (fail secure)
	return false, ""
}

// checkRoleAttributes checks if the user's attributes match the role's attribute requirements.
// This enables Attribute-Based Access Control (ABAC) similar to AWS IAM conditions.
// Currently, attributes are stored with roles but not yet enforced in authorization.
// This is a placeholder for future ABAC implementation.
func checkRoleAttributes(role string, config *Config, req *http.Request) bool {
	// Get role attributes
	roleAttrs, exists := config.roleAttributesMap[role]
	if !exists || roleAttrs == nil {
		// No attributes defined for this role - allow
		return true
	}

	// TODO: Implement attribute checking based on request context
	// For now, if role has attributes defined, we allow access
	// Future implementation can check:
	// - Department from request headers/context
	// - Region from request IP/headers
	// - Environment from request headers/context
	// - Custom attributes from request context

	return true
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

// getEffectiveRolesFromConfig gets all effective roles including inherited ones.
func getEffectiveRolesFromConfig(role string, config *Config) []string {
	effectiveRoles := []string{role}
	visited := make(map[string]bool)
	visited[role] = true

	// Find role definition
	for _, roleDef := range config.Roles {
		if roleDef.Name == role {
			// Recursively get inherited roles
			if len(roleDef.InheritsFrom) > 0 {
				for _, inheritedRole := range roleDef.InheritsFrom {
					if !visited[inheritedRole] {
						visited[inheritedRole] = true
						effectiveRoles = append(effectiveRoles, inheritedRole)
						// Recursively get roles inherited by this role
						inherited := getEffectiveRolesFromConfig(inheritedRole, config)
						for _, ir := range inherited {
							if ir != inheritedRole && !contains(effectiveRoles, ir) {
								effectiveRoles = append(effectiveRoles, ir)
							}
						}
					}
				}
			}
			break
		}
	}

	return effectiveRoles
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

