package main

import (
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// json config path file is required
	rbacConfigs, err := rbac.LoadPermissions("config.json")
	if err != nil {
		return
	}

	overrides := map[string]bool{"user1": true}

	rbacConfigs.OverRides = overrides

	rbacConfigs.RoleExtractorFunc = extractor

	app.UseMiddleware(rbac.Middleware(rbacConfigs))

	app.GET("/sayhello/123", handler)
	app.GET("/greet", rbac.RequireRole("user1", handler))

	app.Run() // listens and serves on localhost:8000
}

func extractor(req *http.Request, _ ...any) (string, error) {
	return req.Header.Get("X-USER-ROLE"), nil
}

func handler(ctx *gofr.Context) (any, error) {
	return "Hello World!", nil
}
