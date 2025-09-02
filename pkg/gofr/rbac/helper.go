package rbac

import "gofr.dev/pkg/gofr"

func HasRole(ctx *gofr.Context, role string) bool {
	expRole, _ := ctx.Context.Value(userRole).(string)
	return expRole == role
}

func GetUserRole(ctx *gofr.Context) string {
	role, _ := ctx.Context.Value(userRole).(string)
	return role
}
