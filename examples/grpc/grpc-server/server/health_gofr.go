package server

import (
	"fmt"
	"google.golang.org/grpc"
	"time"

	"gofr.dev/pkg/gofr"

	gofrGRPC "gofr.dev/pkg/gofr/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type healthServer struct {
	*health.Server
}

var globalHealthServer *healthServer
var healthServerRegistered bool // Global flag to track if health server is registered

// getOrCreateHealthServer ensures only one health server is created and reused.
func getOrCreateHealthServer() *healthServer {
	if globalHealthServer == nil {
		globalHealthServer = &healthServer{health.NewServer()}
	}
	return globalHealthServer
}

func registerServerWithGofr(app *gofr.App, srv any, registerFunc func(grpc.ServiceRegistrar, any)) {
	var s grpc.ServiceRegistrar = app
	h := getOrCreateHealthServer()

	// Register metrics and health server only once
	if !healthServerRegistered {
		gRPCBuckets := []float64{0.005, 0.01, .05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
		app.Metrics().NewHistogram("app_gRPC-Server_stats", "Response time of gRPC server in milliseconds.", gRPCBuckets...)

		healthpb.RegisterHealthServer(s, h.Server)
		h.Server.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		healthServerRegistered = true
	}

	// Register the provided server
	registerFunc(s, srv)
}

func (h *healthServer) Check(ctx *gofr.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	start := time.Now()
	span := ctx.Trace("/grpc.health.v1.Health/Check")
	res, err := h.Server.Check(ctx.Context, req)
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, err, fmt.Sprintf("/grpc.health.v1.Health/Check	Service: %q", req.Service), "app_gRPC-Server_stats")
	span.End()
	return res, err
}

func (h *healthServer) Watch(ctx *gofr.Context, in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	start := time.Now()
	span := ctx.Trace("/grpc.health.v1.Health/Watch")
	err := h.Server.Watch(in, stream)
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, err, fmt.Sprintf("/grpc.health.v1.Health/Watch	Service: %q", in.Service), "app_gRPC-Server_stats")
	span.End()
	return err
}

func (h *healthServer) SetServingStatus(ctx *gofr.Context, service string, servingStatus healthpb.HealthCheckResponse_ServingStatus) {
	start := time.Now()
	span := ctx.Trace("/grpc.health.v1.Health/SetServingStatus")
	h.Server.SetServingStatus(service, servingStatus)
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, nil, fmt.Sprintf("/grpc.health.v1.Health/SetServingStatus	Service:%q", service), "app_gRPC-Server_stats")
	span.End()
}

func (h *healthServer) Shutdown(ctx *gofr.Context) {
	start := time.Now()
	span := ctx.Trace("/grpc.health.v1.Health/Shutdown")
	h.Server.Shutdown()
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, nil, "/grpc.health.v1.Health/Shutdown", "app_gRPC-Server_stats")
	span.End()
}

func (h *healthServer) Resume(ctx *gofr.Context) {
	start := time.Now()
	span := ctx.Trace("/grpc.health.v1.Health/Resume")
	h.Server.Resume()
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, nil, "/grpc.health.v1.Health/Resume", "app_gRPC-Server_stats")
	span.End()
}
