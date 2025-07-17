package rbac

import "gofr.dev/pkg/gofr"

func HasRole(ctx *gofr.Context, role string) bool {
	authInfo := ctx.GetAuthInfo()
	return authInfo.GetRole() == role
}

func IsAdmin(ctx *gofr.Context) bool {
	return HasRole(ctx, "admin")
}

func GetUserRole(ctx *gofr.Context) string {
	authInfo := ctx.GetAuthInfo()
	return authInfo.GetRole()
}
