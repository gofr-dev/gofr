package rbac

import (
	"path"
)

func isRoleAllowed(role, apiroute string, config *Config) bool {
	if config == nil {
		return false
	}

	var routePermissions []string

	// find the matched route from config
	if config.RouteWithPermissions != nil {
		for route, allowedRoles := range config.RouteWithPermissions {
			if isMatched, _ := path.Match(route, apiroute); isMatched && route != "" {
				// check if override is set for the matched route
				if config.OverRides != nil && config.OverRides[apiroute] {
					return true
				}
				routePermissions = allowedRoles
				break
			}
		}

		// append global permissions if any
		if globalRoles, exists := config.RouteWithPermissions["*"]; exists {
			routePermissions = append(routePermissions, globalRoles...)
		}
	}

	// check if role is in allowed roles for the matched route
	for _, allowedRole := range routePermissions {
		if allowedRole == role || allowedRole == "*" {
			return true
		}
	}

	return false
}
