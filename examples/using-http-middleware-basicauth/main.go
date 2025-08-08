package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
)

func main() {
	a := gofr.New()
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
	a.GET("/hello-basic-auth", testHandler)
	a.Run()
}

func testHandler(_ *gofr.Context) (any, error) {
	return "success", nil
}
