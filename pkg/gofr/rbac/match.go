package rbac

import (
	"path"
)

func isRoleAllowed(role, apiroute string, config *Config) bool {
	if config == nil {
		return false
	}

	// Check if route is overridden (public access)
	if config.OverRides != nil && config.OverRides[apiroute] {
		return true
	}

	routePermissions := findRoutePermissions(apiroute, config)

	return isRoleInPermissions(role, routePermissions)
}

// findRoutePermissions finds the permissions for a given route.
func findRoutePermissions(apiroute string, config *Config) []string {
	if config.RouteWithPermissions == nil {
		return nil
	}

	var routePermissions []string

	// Find the matched route from config
	for route, allowedRoles := range config.RouteWithPermissions {
		if isMatched, _ := path.Match(route, apiroute); isMatched && route != "" {
			routePermissions = allowedRoles

			break
		}
	}

	// Append global permissions if any
	if globalRoles, exists := config.RouteWithPermissions["*"]; exists {
		routePermissions = append(routePermissions, globalRoles...)
	}

	return routePermissions
}

// isRoleInPermissions checks if the role is in the allowed permissions.
func isRoleInPermissions(role string, permissions []string) bool {
	for _, allowedRole := range permissions {
		if allowedRole == role || allowedRole == "*" {
			return true
		}
	}

	return false
}
