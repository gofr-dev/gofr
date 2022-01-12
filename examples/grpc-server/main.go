package main

import (
	"github.com/vikash/gofr/examples/grpc-server/grpc"
	"github.com/vikash/gofr/pkg/gofr"
)

func main() {
	app := gofr.New()

	grpc.RegisterHelloServer(app, grpc.Server{})

	app.Run()
}
