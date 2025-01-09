package main

import (
	"gofr.dev/examples/grpc-server/grpc"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	grpc.RegisterHelloServerWithGofr(app, &grpc.HelloGoFrServer{})

	app.Run()
}
