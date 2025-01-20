package main

import (
	"gofr.dev/examples/grpc/grpc-server/server"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	server.RegisterHelloServerWithGofr(app, &server.HelloGoFrServer{})

	app.Run()
}
