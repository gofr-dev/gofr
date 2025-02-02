package server

import (
	"gofr.dev/pkg/gofr"
)

// Register the gRPC service in your app using the following code in your main.go:
//
// server.RegisterHelloServerWithGofr(app, &server.HelloGoFrServer{})
//
// HelloGoFrServer defines the gRPC server implementation.
// Customize the struct with required dependencies and fields as needed.

type HelloGoFrServer struct {
	health *healthServer
}

func (s *HelloGoFrServer) SayHello(ctx *gofr.Context) (any, error) {
	// Uncomment and use the following code if you need to bind the request payload
	// request := HelloRequest{}
	// err := ctx.Bind(&request)
	// if err != nil {
	//     return nil, err
	// }

	//res, err := s.health.Check(ctx, &grpc_health_v1.HealthCheckRequest{
	//	Service: "Hello",
	//})

	//s.health.SetServingStatus(ctx, "Hello", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	return &HelloResponse{}, nil
}
