package rbac

import "gofr.dev/pkg/gofr"

func HasRole(ctx *gofr.Context, role string) bool {
	expRole, _ := ctx.Context.Value(userRole).(string)
	return expRole == role
}

func IsAdmin(ctx *gofr.Context) bool {
	return HasRole(ctx, "admin")
}

func GetUserRole(ctx *gofr.Context) string {
	role, _ := ctx.Context.Value(userRole).(string)
	return role
}
