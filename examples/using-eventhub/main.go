package main

import (
	"gofr.dev/examples/using-eventhub/handlers"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	app.GET("/pub", handlers.Producer)
	app.GET("/sub", handlers.Consumer)

	app.Server.HTTP.Port = 9113
	app.Start()
}
