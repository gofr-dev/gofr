package main

import (
	"gofr.dev/examples/sample-websocket/handlers"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	app.GET("/", handlers.HomeHandler)
	app.GET("/ws", handlers.WSHandler)

	app.Server.WSUpgrader.WriteBufferSize = 4096

	app.Start()
}
