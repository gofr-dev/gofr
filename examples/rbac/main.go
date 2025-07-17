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

	rbacConfigs.RoleExtractorFunc = extractor

	app.UseMiddleware(rbac.Middleware(rbacConfigs))

	handler := func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	}

	app.GET("/sayhello/123", handler)
	app.GET("/greet", rbac.RequireRole("admin", handler))

	app.Run() // listens and serves on localhost:8000
}

func extractor(req *http.Request, _ ...any) (string, error) {
	return req.Header.Get("X-USER-ROLE"), nil
}
