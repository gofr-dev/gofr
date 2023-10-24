package main

import (
	"gofr.dev/examples/sample-https/handler"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// add a handler
	app.GET("/hello-world", handler.HelloWorld)
	app.GET("/hello", handler.HelloName)
	app.POST("/post/", handler.PostName)
	app.GET("/error", handler.ErrorHandler)
	app.GET("/multiple-errors", handler.MultipleErrorHandler)

	// set http redirect to https
	app.Server.HTTP.RedirectToHTTPS = true

	app.Start()
}
