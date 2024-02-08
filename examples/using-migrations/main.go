package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
	gofrMigration "gofr.dev/pkg/gofr/migration"

	"gofr.dev/examples/using-migrations/migrations"
)

func main() {
	// Create a new application
	a := gofr.New()

	// Add migrations to run
	a.Migrate(gofrMigration.SQL(), migrations.All())

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
