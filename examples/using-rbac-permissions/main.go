package main

import (
	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// OAuth verifies the token and puts its claims in the request; RBAC then reads the "scope"
	// claim named in configs/rbac.json. OAuth must be enabled first.
	//
	// Permissions mode trusts what the token says, so the issuer is checked here: a token from any
	// other issuer, or with no "iss", gets a 401 even if it is signed by a key in the same JWKS.
	if err := app.EnableOAuth(app.Config.Get("JWKS_URL"), 10, jwt.WithIssuer(app.Config.Get("TOKEN_ISSUER"))); err != nil {
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
