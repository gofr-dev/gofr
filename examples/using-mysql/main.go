package main

import (
	"gofr.dev/examples/using-mysql/handler"
	"gofr.dev/examples/using-mysql/store"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	s := store.New()
	h := handler.New(s)

	app.Server.ValidateHeaders = false

	// specifying the different routes supported by this service
	app.GET("/employee", h.Get)
	app.POST("/employee", h.Create)

	app.Server.HTTP.Port = 9001
	app.Server.MetricsPort = 2113

	app.Start()
}
