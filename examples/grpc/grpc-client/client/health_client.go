// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
// versions:
// 	gofr-cli v0.6.0
// 	gofr.dev v1.37.0
// 	source: hello.proto

package client

import (
	"fmt"
	"sync"
	"time"

	"gofr.dev/pkg/gofr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	gofrgRPC "gofr.dev/pkg/gofr/grpc"
)

var (
	metricsOnce sync.Once
	gRPCBuckets = []float64{0.005, 0.01, .05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
)

type HealthClient interface {
	Check(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (*grpc_health_v1.HealthCheckResponse, error)
	Watch(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (
	grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse], error)
}

type HealthClientWrapper struct {
	client grpc_health_v1.HealthClient
}

func NewHealthClient(conn *grpc.ClientConn) HealthClient {
	return &HealthClientWrapper{
		client: grpc_health_v1.NewHealthClient(conn),
	}
}

func createGRPCConn(host string, serviceName string) (*grpc.ClientConn, error) {
	serviceConfig := `{"loadBalancingPolicy": "round_robin"}`

	conn, err := grpc.Dial(host,
		grpc.WithDefaultServiceConfig(serviceConfig),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func invokeRPC(ctx *gofr.Context, rpcName string, rpcFunc func() (interface{}, error)) (interface{}, error) {
	span := ctx.Trace("gRPC-srv-call: " + rpcName)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	md := metadata.Pairs("x-gofr-traceid", traceID, "x-gofr-spanid", spanID)

	ctx.Context = metadata.NewOutgoingContext(ctx.Context, md)
	transactionStartTime := time.Now()

	res, err := rpcFunc()
	logger := gofrgRPC.NewgRPCLogger()
	logger.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), transactionStartTime, err,
	rpcName, "app_gRPC-Client_stats")

	return res, err
}

func (h *HealthClientWrapper) Check(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, 
	opts ...grpc.CallOption) (*grpc_health_v1.HealthCheckResponse, error) {
	result, err := invokeRPC(ctx, fmt.Sprintf("/grpc.health.v1.Health/Check	Service: %q", in.Service), func() (interface{}, error) {
		return h.client.Check(ctx, in, opts...)
	})

	if err != nil {
		return nil, err
	}
	return result.(*grpc_health_v1.HealthCheckResponse), nil
}

func (h *HealthClientWrapper) Watch(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, 
	opts ...grpc.CallOption) (grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse], error) {
	result, err := invokeRPC(ctx, fmt.Sprintf("/grpc.health.v1.Health/Watch	Service: %q", in.Service), func() (interface{}, error) {
		return h.client.Watch(ctx, in, opts...)
	})

	if err != nil {
		return nil, err
	}

	return result.(grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse]), nil
}
