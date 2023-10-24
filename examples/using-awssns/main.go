package main

import (
	"gofr.dev/examples/using-awssns/handlers"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.Server.ValidateHeaders = false

	app.POST("/publish", handlers.Publisher)
	app.GET("/subscribe", handlers.Subscriber)

	app.Start()
}
