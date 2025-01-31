package main

import (
	"fmt"
	"gofr.dev/examples/grpc/grpc-client/client"
	"gofr.dev/pkg/gofr"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	app := gofr.New()

	// Create a gRPC client for the Hello service
	helloGRPCClient, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
	if err != nil {
		app.Logger().Errorf("Failed to create Hello gRPC client: %v", err)
		return
	}

	greet := NewGreetHandler(helloGRPCClient)

	app.POST("/hello", greet.Hello)

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

	// HealthCheck to SayHello Service.
	res, err := g.helloGRPCClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "Hello"})
	if err != nil {
		return nil, err
	} else if res.Status == grpc_health_v1.HealthCheckResponse_NOT_SERVING {
		ctx.Error("SayHello Service is down")
		return nil, fmt.Errorf("SayHello Service is down")
	}

	// Make a gRPC call to the Hello service
	helloResponse, err := g.helloGRPCClient.SayHello(ctx, &client.HelloRequest{Name: userName})
	if err != nil {
		return nil, err
	}

	return helloResponse, nil
}
