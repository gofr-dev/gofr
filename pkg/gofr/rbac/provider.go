package rbac

import (
	"net/http"

	"gofr.dev/pkg/gofr/container"
)

// Provider implements container.RBACProvider interface.
// This follows the same pattern as datasource providers (e.g., mongo.Client).
type Provider struct{}

// NewProvider creates a new RBAC provider.
// This follows the same pattern as datasource providers (e.g., mongo.New()).
//
// Example:
//
//	provider := rbac.NewProvider()
//	app.EnableRBAC(provider, "configs/rbac.json")
func NewProvider() container.RBACProvider {
	return &Provider{}
}

// LoadPermissions loads RBAC configuration from a file.
func (p *Provider) LoadPermissions(file string) (any, error) {
	return LoadPermissions(file)
}

// GetMiddleware returns the middleware function for the given config.
func (p *Provider) GetMiddleware(config any) func(http.Handler) http.Handler {
	rbacConfig, ok := config.(*Config)
	if !ok {
		// If config is not *Config, return passthrough middleware
		return func(handler http.Handler) http.Handler {
			return handler
		}
	}

	return Middleware(rbacConfig)
}

// RequireRole wraps a handler to require a specific role.
func (p *Provider) RequireRole(allowedRole string, handlerFunc func(any) (any, error)) func(any) (any, error) {
	hf := HandlerFunc(handlerFunc)
	wrapped := RequireRole(allowedRole, hf)

	return func(ctx any) (any, error) {
		return wrapped(ctx)
	}
}

// RequireAnyRole wraps a handler to require any of the specified roles.
func (p *Provider) RequireAnyRole(allowedRoles []string, handlerFunc func(any) (any, error)) func(any) (any, error) {
	hf := HandlerFunc(handlerFunc)
	wrapped := RequireAnyRole(allowedRoles, hf)

	return func(ctx any) (any, error) {
		return wrapped(ctx)
	}
}

// RequirePermission wraps a handler to require a specific permission.
func (p *Provider) RequirePermission(requiredPermission string, permissionConfig any, handlerFunc func(any) (any, error)) func(any) (any, error) {
	var rbacPermConfig *PermissionConfig

	// Convert permissionConfig to *PermissionConfig
	if permissionConfig == nil {
		rbacPermConfig = &PermissionConfig{}
	} else {
		// Try to convert from gofr.PermissionConfig interface
		if pc, ok := permissionConfig.(interface {
			GetRolePermissions() map[string][]string
			GetRoutePermissionMap() map[string]string
			GetPermissions() map[string][]string
			GetRoutePermissionRules() []RoutePermissionRule
		}); ok {
			rules := pc.GetRoutePermissionRules()
			if len(rules) > 0 {
				rbacRules := make([]RoutePermissionRule, len(rules))
				for i, rule := range rules {
					rbacRules[i] = RoutePermissionRule{
						Methods:    rule.Methods,
						Path:       rule.Path,
						Regex:      rule.Regex,
						Permission: rule.Permission,
					}
				}
				rbacPermConfig = &PermissionConfig{
					RoutePermissionRules: rbacRules,
					RolePermissions:      pc.GetRolePermissions(),
				}
			} else {
				rbacPermConfig = &PermissionConfig{
					RolePermissions:    pc.GetRolePermissions(),
					RoutePermissionMap: pc.GetRoutePermissionMap(),
					Permissions:        pc.GetPermissions(),
				}
			}
		} else if pc, ok := permissionConfig.(*PermissionConfig); ok {
			rbacPermConfig = pc
		} else {
			rbacPermConfig = &PermissionConfig{}
		}
	}

	hf := HandlerFunc(handlerFunc)
	wrapped := RequirePermission(requiredPermission, rbacPermConfig, hf)

	return func(ctx any) (any, error) {
		return wrapped(ctx)
	}
}

// ErrAccessDenied returns the error used when access is denied.
func (p *Provider) ErrAccessDenied() error {
	return ErrAccessDenied
}

// ErrPermissionDenied returns the error used when permission is denied.
func (p *Provider) ErrPermissionDenied() error {
	return ErrPermissionDenied
}

