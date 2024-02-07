package main

import (
	"fmt"
	"gofr.dev/examples/using-migrations/migrations"
	"gofr.dev/pkg/gofr"

	migrate "gofr.dev/pkg/gofr/migrations"
)

func main() {
	// Create a new application
	a := gofr.New()

	a.Migrate(migrations.All(), migrate.NewSQL())

	// Add all the routes
	a.GET("/hello", HelloHandler)

	// Run the application
	a.Run()
}

func HelloHandler(c *gofr.Context) (interface{}, error) {
	name := c.Param("name")
	if name == "" {
		c.Log("Name came empty")
		name = "World"
	}

	return fmt.Sprintf("Hello %s!", name), nil
}
