package grpc

import (
	context "context"

	"fmt"
	"gofr.dev/pkg/gofr/container"
)

type Server struct {
	UnimplementedHelloServer
	Cont *container.Container
}

func (s *Server) SayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	name := req.Name
	if name == "" {
		name = "World"
	}

	s.Cont.Logger.Debug("container injected!")

	return &HelloResponse{
		Message: fmt.Sprintf("Hello %s!", name),
	}, nil
}
