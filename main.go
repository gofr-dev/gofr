package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Register a startup hook
	app.OnStart(func(a *gofr.App) error {
		fmt.Println(">>> Startup hook executed!")
		// You can do any initialization here
		return nil // or return an error to test error handling
	})

	app.GET("/greet", func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	})

	app.Run()
}
