package main

import (
	"gofr.dev/examples/using-mongo/handlers"
	"gofr.dev/examples/using-mongo/stores/customer"
	"gofr.dev/pkg/gofr"
)

func main() {
	// create the application object
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	store := customer.New()
	h := handlers.New(store)

	// specifying the different routes supported by this service
	app.GET("/customer", h.Get)
	app.POST("/customer", h.Create)
	app.DELETE("/customer", h.Delete)
	app.Server.HTTP.Port = 8097

	app.Start()
}
