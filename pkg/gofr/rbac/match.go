package rbac

import "path"

func isPathAllowed(userRoles []string, route string, config *Config) bool {
	for _, role := range userRoles {
		allowedPaths, exists := config.RoleWithPermissions[role]
		if !exists {
			continue
		}
		for _, p := range allowedPaths {
			if ok, _ := path.Match(p, route); ok {
				return true
			}
		}
	}
	return false
}
