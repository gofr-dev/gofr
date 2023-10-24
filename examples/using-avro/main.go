package main

import (
	"gofr.dev/examples/using-avro/handlers"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	app.GET("/pub", handlers.Producer)
	app.GET("/sub", handlers.Consumer)

	app.Server.HTTP.Port = 9111
	app.Start()
}
