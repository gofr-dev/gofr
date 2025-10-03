package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
)

func main() {
	a := gofr.New()

	// For Basic Auth
	//setupBasicAuth(a)

	// For APIKey Auth
	//setupAPIKeyAuth(a)

	//For OAuth
	//a.EnableOAuth("<JWKS-Endpoint>", 10)

	a.GET("/test-auth", testHandler)
	a.Run()
}

func testHandler(_ *gofr.Context) (any, error) {
	return "success", nil
}

func setupBasicAuth(a *gofr.App) {
	a.EnableBasicAuthWithValidator(func(c *container.Container, username, password string) bool {
		if username == "username" && password == "password" {
			return true
		}
		return false
		// Alternatively, get the expected username/password from any storage and validate
		//expectedPassword, err := c.KVStore.Get(context.Background(), username)
		//if err != nil || expectedPassword != password {
		//	return false
		//}
		//return true

	})
}

func setupAPIKeyAuth(a *gofr.App) {
	a.EnableAPIKeyAuthWithValidator(func(c *container.Container, apiKey string) bool {
		// basic validation based on fixed set of credentials
		return apiKey == "valid-api-key"

		// Alternatively, get the expected APIKey from any storage and validate
		//data, err := c.KVStore.Get(context.Background(), apiKey)
		//if err != nil || data == "" {
		//	return false
		//}
		//return true
	})
}
