package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	rbacConfigs, err := rbac.LoadPermissions("path-to-file")
	if err != nil {
		return
	}

	rbac.Middleware(rbacConfigs)

	handler := func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	}

	app.GET("/greet", rbac.RequireRole("any", handler))

	app.Run() // listens and serves on localhost:8000
}
