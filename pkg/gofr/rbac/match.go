package rbac

import (
	"path"
)

func isRoleAllowed(role, apiroute string, config *Config) bool {
	var routePermissions []string

	// find the matched route from config
	for route, allowedRoles := range config.RouteWithPermissions {
		if isMatched, _ := path.Match(route, apiroute); isMatched && route != "" {
			// check if override is set for the matched route
			if config.OverRides[apiroute] {
				return true
			}
			routePermissions = allowedRoles
			break
		}
	}

	// append global permissions if any
	routePermissions = append(routePermissions, config.RouteWithPermissions["*"]...)

	// check if role is in allowed roles for the matched route
	for _, allowedRole := range routePermissions {
		if allowedRole == role || allowedRole == "*" {
			return true
		}
	}

	return false
}
