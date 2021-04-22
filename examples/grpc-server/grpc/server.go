package grpc

import (
	context "context"
	"fmt"
)

type Server struct {
	UnimplementedHelloServer
}

func (Server) SayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	name := req.Name
	if name == "" {
		name = "World"
	}

	return &HelloResponse{
		Message: fmt.Sprintf("Hello %s", name),
	}, nil
}
