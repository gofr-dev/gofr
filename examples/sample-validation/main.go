package main

import (
	"gofr.dev/examples/sample-validation/handler"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	app.POST("/phone", handler.ValidateEntry)

	app.Start()
}
