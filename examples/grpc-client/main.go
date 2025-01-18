package main

import (
	"gofr.dev/examples/grpc-client/grpc"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	grpc.RegisterHelloServerWithGofr(app, &grpc.HelloGoFrServer{})

	app.Run()
}
