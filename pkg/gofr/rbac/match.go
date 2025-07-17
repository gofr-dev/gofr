package rbac

import (
	"path"
	"strings"
)

func isPathAllowed(role string, route string, config *Config) bool {
	allowedPaths := config.RoleWithPermissions[role]

	for _, pattern := range allowedPaths {
		// Allow simple wildcard "*"
		if pattern == "*" {
			return true
		}

		// Ensure pattern ends with * if it's a prefix match
		if pattern == route {
			return true
		}
		// Normalize pattern and path to avoid trailing slash issues
		normalizedPattern := strings.TrimSuffix(pattern, "/")
		normalizedPath := strings.TrimSuffix(route, "/")

		// Allow matching wildcard like /admin/* or /users/*
		if ok, _ := path.Match(normalizedPattern, normalizedPath); ok {
			return true
		}

		// Support prefix match with * (e.g. /users/* should match /users/123)
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(route, prefix) {
				return true
			}
		}
	}

	return false
}
