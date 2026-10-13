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
	// RBAC does not validate the token, and permissions mode trusts what it says, so every check
	// belongs here, where it runs on every request that carries a token: a token from another issuer,
	// minted for another API, or with no expiry gets a 401 even if a key in the same JWKS signed it.
	if err := app.EnableOAuth(app.Config.Get("JWKS_URL"), 10,
		jwt.WithIssuer(app.Config.Get("TOKEN_ISSUER")),
		jwt.WithAudience("orders-api"),
		jwt.WithExpirationRequired(),
	); err != nil {
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
