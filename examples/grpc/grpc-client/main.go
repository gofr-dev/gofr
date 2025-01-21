package main

import (
	"gofr.dev/examples/grpc/grpc-client/client"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Create a gRPC client for the Hello service
	helloGRPCClient, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"))
	if err != nil {
		app.Logger().Errorf("Failed to create Hello gRPC client: %v", err)
		return
	}

	greet := NewGreetHandler(helloGRPCClient)

	app.GET("/hello", greet.Hello)

	app.Run()
}

type GreetHandler struct {
	helloGRPCClient client.HelloGoFrClient
}

func NewGreetHandler(helloClient client.HelloGoFrClient) *GreetHandler {
	return &GreetHandler{
		helloGRPCClient: helloClient,
	}
}

func (g GreetHandler) Hello(ctx *gofr.Context) (interface{}, error) {
	userName := ctx.Param("name")

	if userName == "" {
		ctx.Log("Name parameter is empty, defaulting to 'World'")
		userName = "World"
	}

	// Make a gRPC call to the Hello service
	helloResponse, err := g.helloGRPCClient.SayHello(ctx, &client.HelloRequest{Name: userName})
	if err != nil {
		return nil, err
	}

	return helloResponse, nil
}