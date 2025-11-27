package rbac

import (
	"path"
	"strings"
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
	if config == nil || config.RouteWithPermissions == nil {
		return nil
	}

	routePermissions, matchedSpecificRoute := findSpecificRouteMatch(apiroute, config)

	// Only append global permissions if no specific route matched
	// This ensures specific routes take precedence over global wildcard
	if !matchedSpecificRoute {
		routePermissions = appendGlobalPermissions(routePermissions, config)
	}

	return routePermissions
}

// findSpecificRouteMatch finds if the route matches any specific route pattern.
func findSpecificRouteMatch(apiroute string, config *Config) ([]string, bool) {
	for route, allowedRoles := range config.RouteWithPermissions {
		// Skip global wildcard for now - we'll handle it separately
		if route == "*" {
			continue
		}

		if matchesRoute(route, apiroute) {
			return allowedRoles, true
		}
	}

	return nil, false
}

// matchesRoute checks if a route pattern matches the given API route.
func matchesRoute(route, apiroute string) bool {
	// Check if route pattern matches using path.Match
	if isMatched, _ := path.Match(route, apiroute); isMatched && route != "" {
		return true
	}

	// Also check if route is a prefix match for patterns ending with /*
	// e.g., /api/admin should match /api/admin/*
	if strings.HasSuffix(route, "/*") {
		prefix := strings.TrimSuffix(route, "/*")

		return apiroute == prefix || strings.HasPrefix(apiroute, prefix+"/")
	}

	return false
}

// appendGlobalPermissions appends global wildcard permissions if they exist.
func appendGlobalPermissions(routePermissions []string, config *Config) []string {
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
