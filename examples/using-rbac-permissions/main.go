package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// OAuth verifies the token and puts its claims in the request; RBAC then reads the "scope"
	// claim named in configs/rbac.json. OAuth must be enabled first.
	if err := app.EnableOAuth(app.Config.Get("JWKS_URL"), 10); err != nil {
		app.Logger().Fatalf("%v", err)
	}

	if err := app.EnableRBAC(); err != nil {
		app.Logger().Fatalf("%v", err)
	}

	app.GET("/orders", listOrders)
	app.POST("/orders", createOrder)

	app.Run()
}

func listOrders(_ *gofr.Context) (any, error) {
	return []string{"order-1", "order-2"}, nil
}

func createOrder(_ *gofr.Context) (any, error) {
	return "order created", nil
}
