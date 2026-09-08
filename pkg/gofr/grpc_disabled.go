//go:build gofr_nogrpc

package gofr

import (
	"context"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
)

// disabledGRPC stands in for the gRPC server in a build made with
// -tags gofr_nogrpc, which omits google.golang.org/grpc entirely.
//
// It is constructed like the real one, so App's lifecycle is unchanged, but it
// never listens. Nothing reaches Run: App starts the gRPC server only when
// grpcRegistered is set, and the only thing that sets it is RegisterService,
// which does not exist in this build. Shutdown is on the shutdown path
// unconditionally, so it has to be a working no-op rather than a panic.
type disabledGRPC struct{}

func newGRPCRunner(_ *container.Container, _ int, _ config.Config) (grpcRunner, error) {
	return disabledGRPC{}, nil
}

func (disabledGRPC) Run(c *container.Container) {
	c.Logger.Error("gRPC server was started, but this binary was built with -tags gofr_nogrpc, " +
		"which omits the gRPC server. Rebuild without the tag to use it.")
}

func (disabledGRPC) Shutdown(context.Context) error { return nil }
