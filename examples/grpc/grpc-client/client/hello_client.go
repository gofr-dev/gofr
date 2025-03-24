// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package client

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/metrics"
	"google.golang.org/grpc"
)

type HelloGoFrClient interface {
	SayHello(*gofr.Context, *HelloRequest, ...grpc.CallOption) (*HelloResponse, error)
	HealthClient
}

type HelloClientWrapper struct {
	client HelloClient
	HealthClient
}

func NewHelloGoFrClient(host string, metrics metrics.Manager) (HelloGoFrClient, error) {
	conn, err := createGRPCConn(host, "Hello")
	if err != nil {
		return &HelloClientWrapper{
			client:       nil,
			HealthClient: &HealthClientWrapper{client: nil}, // Ensure HealthClient is also implemented
		}, err
	}

	metricsOnce.Do(func() {
		metrics.NewHistogram("app_gRPC-Client_stats", "Response time of gRPC client in milliseconds.", gRPCBuckets...)
	})

	res := NewHelloClient(conn)
	healthClient := NewHealthClient(conn)

	return &HelloClientWrapper{
		client:       res,
		HealthClient: healthClient,
	}, nil
}
func (h *HelloClientWrapper) SayHello(ctx *gofr.Context, req *HelloRequest,
	opts ...grpc.CallOption) (*HelloResponse, error) {
	result, err := invokeRPC(ctx, "/Hello/SayHello", func() (interface{}, error) {
		return h.client.SayHello(ctx.Context, req, opts...)
	})

	if err != nil {
		return nil, err
	}
	return result.(*HelloResponse), nil
}
