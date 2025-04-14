// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
// versions:
// 	gofr-cli v0.6.0
// 	gofr.dev v1.37.0
// 	source: hello.proto

package server

import (
	"context"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// NewHelloGoFrServer creates a new instance of HelloGoFrServer
func NewHelloGoFrServer() *HelloGoFrServer {
	return &HelloGoFrServer{
		health: getOrCreateHealthServer(), // Initialize the health server
	}
}

// HelloServerWithGofr is the interface for the server implementation
type HelloServerWithGofr interface {
	SayHello(*gofr.Context) (any, error)
}

// HelloServerWrapper wraps the server and handles request and response logic
type HelloServerWrapper struct {
	HelloServer
	*healthServer
	Container *container.Container
	server    HelloServerWithGofr
}

//
// SayHello wraps the method and handles its execution
func (h *HelloServerWrapper) SayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	gctx := h.getGofrContext(ctx, &HelloRequestWrapper{ctx: ctx, HelloRequest: req})

	res, err := h.server.SayHello(gctx)
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*HelloResponse)
	if !ok {
		return nil, status.Errorf(codes.Unknown, "unexpected response type %T", res)
	}

	return resp, nil
}

// mustEmbedUnimplementedHelloServer ensures that the server implements all required methods
func (h *HelloServerWrapper) mustEmbedUnimplementedHelloServer() {}

// RegisterHelloServerWithGofr registers the server with the application
func RegisterHelloServerWithGofr(app *gofr.App, srv HelloServerWithGofr) {
	registerServerWithGofr(app, srv, func(s grpc.ServiceRegistrar, srv any) {
		wrapper := &HelloServerWrapper{server: srv.(HelloServerWithGofr), healthServer: getOrCreateHealthServer()}
		RegisterHelloServer(s, wrapper)
		wrapper.Server.SetServingStatus("Hello", healthpb.HealthCheckResponse_SERVING)
	})
}

// getGofrContext extracts the GoFr context from the original context
func (h *HelloServerWrapper) getGofrContext(ctx context.Context, req gofr.Request) *gofr.Context {
	return &gofr.Context{
		Context:   ctx,
		Container: h.Container,
		Request:   req,
	}
}
