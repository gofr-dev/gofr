package server

import (
	"fmt"

	"gofr.dev/pkg/gofr"
)

// Register the gRPC service in your app using the following code in your main.go:
//
// client.RegisterHelloServerWithGofr(app, &client.HelloGoFrServer{})
//
// HelloGoFrServer defines the gRPC client implementation.
// Customize the struct with required dependencies and fields as needed.

type HelloGoFrServer struct {
}

func (s *HelloGoFrServer) SayHello(ctx *gofr.Context) (any, error) {
	request := HelloRequest{}

	err := ctx.Bind(&request)
	if err != nil {
		return nil, err
	}

	name := request.Name
	if name == "" {
		name = "World"
	}

	return &HelloResponse{
		Message: fmt.Sprintf("Hello %s!", name),
	}, nil
}
