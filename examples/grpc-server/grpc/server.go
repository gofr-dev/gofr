package grpc

import (
	context "context"

	"fmt"
	"gofr.dev/pkg/gofr/container"
)

type Server struct {
	// conatiner can be emebed into the server struct
	// to access the datasource and logger functionalities
	*container.Container

	UnimplementedHelloServer
}

func (s *Server) SayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	name := req.Name
	if name == "" {
		name = "World"
	}

	s.Logger.Debug("container injected!")

	return &HelloResponse{
		Message: fmt.Sprintf("Hello %s!", name),
	}, nil
}
