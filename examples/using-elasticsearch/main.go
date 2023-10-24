package main

import (
	"gofr.dev/examples/using-elasticsearch/handler"
	"gofr.dev/examples/using-elasticsearch/store/customer"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.Server.ValidateHeaders = false

	store := customer.New("customers")
	h := handler.New(store)

	app.REST("customer", h)

	app.Start()
}
