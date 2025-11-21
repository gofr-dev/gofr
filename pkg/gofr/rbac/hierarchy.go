package rbac

import (
	"context"
	"sync"
)

// RoleHierarchy manages role inheritance relationships.
// Example: admin > editor > author > viewer
type RoleHierarchy struct {
	// hierarchy maps roles to their inherited roles
	// Example: "admin": ["editor", "author", "viewer"]
	hierarchy map[string][]string
	mu        sync.RWMutex
}

// NewRoleHierarchy creates a new role hierarchy from a map.
func NewRoleHierarchy(hierarchy map[string][]string) *RoleHierarchy {
	if hierarchy == nil {
		hierarchy = make(map[string][]string)
	}

	return &RoleHierarchy{
		hierarchy: hierarchy,
	}
}

// GetEffectiveRoles returns all roles that the given role inherits.
// This includes the role itself and all inherited roles.
func (h *RoleHierarchy) GetEffectiveRoles(role string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if role == "" {
		return []string{}
	}

	// Start with the role itself
	effectiveRoles := []string{role}
	visited := make(map[string]bool)
	visited[role] = true

	// Get inherited roles recursively
	h.getInheritedRoles(role, &effectiveRoles, visited)

	return effectiveRoles
}

// getInheritedRoles recursively collects all inherited roles.
func (h *RoleHierarchy) getInheritedRoles(role string, effectiveRoles *[]string, visited map[string]bool) {
	inherited, exists := h.hierarchy[role]
	if !exists {
		return
	}

	for _, inheritedRole := range inherited {
		if !visited[inheritedRole] {
			visited[inheritedRole] = true
			*effectiveRoles = append(*effectiveRoles, inheritedRole)
			// Recursively get roles inherited by this role
			h.getInheritedRoles(inheritedRole, effectiveRoles, visited)
		}
	}
}

// HasRole checks if the user's role (or any inherited role) matches the required role.
func (h *RoleHierarchy) HasRole(ctx context.Context, requiredRole string) bool {
	role, _ := ctx.Value(userRole).(string)
	if role == "" {
		return false
	}

	// Direct match
	if role == requiredRole {
		return true
	}

	// Check if required role is in the effective roles (inherited)
	effectiveRoles := h.GetEffectiveRoles(role)
	for _, effectiveRole := range effectiveRoles {
		if effectiveRole == requiredRole {
			return true
		}
	}

	return false
}

// HasAnyRole checks if the user's role (or any inherited role) matches any of the required roles.
func (h *RoleHierarchy) HasAnyRole(ctx context.Context, requiredRoles []string) bool {
	role, _ := ctx.Value(userRole).(string)
	if role == "" {
		return false
	}

	effectiveRoles := h.GetEffectiveRoles(role)

	// Check if any required role matches
	for _, requiredRole := range requiredRoles {
		// Direct match
		if role == requiredRole {
			return true
		}

		// Check inherited roles
		for _, effectiveRole := range effectiveRoles {
			if effectiveRole == requiredRole {
				return true
			}
		}
	}

	return false
}

// IsRoleAllowedWithHierarchy checks if a role is allowed, considering hierarchy.
func IsRoleAllowedWithHierarchy(role, route string, config *Config, hierarchy *RoleHierarchy) bool {
	if hierarchy == nil {
		// Fallback to regular role check
		return isRoleAllowed(role, route, config)
	}

	// Get effective roles (including inherited)
	effectiveRoles := hierarchy.GetEffectiveRoles(role)

	// Check if any effective role is allowed
	for _, effectiveRole := range effectiveRoles {
		if isRoleAllowed(effectiveRole, route, config) {
			return true
		}
	}

	return false
}

