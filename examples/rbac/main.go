package main

import (
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// loading the rbac config file which is required
	rbacConfigs, err := rbac.LoadPermissions("config.json")
	if err != nil {
		return
	}

	// example of setting override for a specific role
	overrides := map[string]bool{"/greet": true}
	rbacConfigs.OverRides = overrides

	// setting the role extractor function
	rbacConfigs.RoleExtractorFunc = extractor

	// applying the middleware
	app.UseMiddleware(rbac.Middleware(rbacConfigs))

	// sample routes
	app.GET("/sayhello/321", handler)
	app.GET("/greet", rbac.RequireRole("user1", handler))

	app.Run() // listens and serves on localhost:8000
}

func extractor(req *http.Request, _ ...any) (string, error) {
	return req.Header.Get("X-USER-ROLE"), nil
}

func handler(ctx *gofr.Context) (any, error) {
	return "Hello World!", nil
}
