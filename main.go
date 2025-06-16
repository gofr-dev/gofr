package main

import (
	"fmt"
	"log"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Register a startup hook
	app.OnStart(func(a *gofr.App) error {
		fmt.Println(">>> Startup hook executed!")

		return nil
	})

	app.GET("/greet", func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	})

	if err := app.Run(); err != nil {
		log.Fatalf("app failed: %v", err)
	}
}
