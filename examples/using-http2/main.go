package main

import (
	"gofr.dev/examples/using-http2/handler"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	// add a handler
	app.GET("/static/{name}", handler.ServeStatic)
	app.GET("/home", handler.HomeHandler)

	// set https port and redirect
	app.Server.HTTPS.Port = 1449
	app.Server.HTTP.RedirectToHTTPS = false

	// http port
	app.Server.HTTP.Port = 9017

	app.Start()
}
