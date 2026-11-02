//go:build gofr_nogrpc

package gofr

import (
	"context"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
)

// grpcServer stands in for the gRPC server in a build made with
// -tags gofr_nogrpc, which omits google.golang.org/grpc entirely.
//
// It has the same name as the real type in grpc.go, so App holds a concrete
// *grpcServer in every build and never type-asserts to reach it. Only Run and
// Shutdown exist here: the setters that take gRPC types -- AddGRPCServerOptions,
// the interceptor setters, RegisterService -- live in grpc.go and are compiled
// out with it. A user who calls them and also sets the tag gets a compile error,
// which is the honest outcome: they asked for a binary without gRPC and then
// used gRPC.
//
// It is constructed like the real one, so App's lifecycle is unchanged, but it
// never listens. Nothing reaches Run: App starts the gRPC server only when
// grpcRegistered is set, and the only thing that sets it is RegisterService,
// which does not exist in this build. Shutdown is on the shutdown path
// unconditionally, so it has to be a working no-op rather than a panic.
type grpcServer struct{}

func newGRPCServer(_ *container.Container, _ int, _ config.Config) (*grpcServer, error) {
	return &grpcServer{}, nil
}

func (*grpcServer) Run(c *container.Container) {
	c.Logger.Error("gRPC server was started, but this binary was built with -tags gofr_nogrpc, " +
		"which omits the gRPC server. Rebuild without the tag to use it.")
}

func (*grpcServer) Shutdown(context.Context) error { return nil }
