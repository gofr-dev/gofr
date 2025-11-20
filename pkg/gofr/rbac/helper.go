package rbac

import "context"

// ContextValueGetter is an interface for accessing context values.
// This avoids import cycle with gofr package.
type ContextValueGetter interface {
	Value(key interface{}) interface{}
}

// HasRole checks if the context contains the specified role.
// ctx should be a *gofr.Context or any type that implements Value(key interface{}) interface{}
func HasRole(ctx ContextValueGetter, role string) bool {
	if ctx == nil {
		return false
	}

	expRole, _ := ctx.Value(userRole).(string)
	return expRole == role
}

// GetUserRole extracts the user role from the context.
// ctx should be a *gofr.Context or any type that implements Value(key interface{}) interface{}
func GetUserRole(ctx ContextValueGetter) string {
	if ctx == nil {
		return ""
	}

	role, _ := ctx.Value(userRole).(string)
	return role
}

// HasRoleFromContext is a convenience function that works with standard context.Context.
func HasRoleFromContext(ctx context.Context, role string) bool {
	expRole, _ := ctx.Value(userRole).(string)
	return expRole == role
}

// GetUserRoleFromContext is a convenience function that works with standard context.Context.
func GetUserRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(userRole).(string)
	return role
}
