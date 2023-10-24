package main

import (
	handlers "gofr.dev/examples/using-dynamodb/handlers/person"
	stores "gofr.dev/examples/using-dynamodb/stores/person"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	s := stores.New("person")
	h := handlers.New(s)

	app.GET("/person/{id}", h.GetByID)
	app.POST("/person", h.Create)
	app.PUT("/person/{id}", h.Update)
	app.DELETE("/person/{id}", h.Delete)

	app.Start()
}
