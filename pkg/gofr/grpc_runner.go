package gofr

import (
	"context"

	"gofr.dev/pkg/gofr/container"
)

// grpcRunner is everything App needs from the gRPC server once it has been
// built: start it, and stop it.
//
// The field on App is this interface rather than the concrete *grpcServer so
// that the implementation -- and with it google.golang.org/grpc, its reflection
// and health services and the recovery middleware -- can be left out of a build
// that serves no gRPC. As with GraphQL, the interface is only half of it:
// grpc.go carries the build tag too, because an interface field does not stop
// the concrete file from compiling the library in.
//
// The methods that take gRPC types -- AddGRPCServerOptions, the interceptor
// setters, RegisterService -- deliberately stay off this interface and live in
// grpc.go. They cannot be named here without importing google.golang.org/grpc,
// which is the import this whole arrangement exists to make optional. A user who
// calls them and also sets -tags gofr_nogrpc gets a compile error, which is the
// honest outcome: they asked for a binary without gRPC and then used gRPC.
//
// newGRPCRunner, which builds one, likewise has two definitions selected by the
// same tag: the real one in grpc.go, a stub in grpc_disabled.go.
type grpcRunner interface {
	Run(c *container.Container)
	Shutdown(ctx context.Context) error
}
