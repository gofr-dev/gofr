// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package server

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HelloServerWithGofr interface {
	SayHello(*gofr.Context) (any, error)
}

type HelloServerWrapper struct {
	HelloServer
	Container *container.Container
	server    HelloServerWithGofr
}
func (h *HelloServerWrapper) SayHello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	gctx := h.GetGofrContext(ctx, &HelloRequestWrapper{ctx: ctx, HelloRequest: req})

	start := time.Now()

	res, err := h.server.SayHello(gctx)
	if err != nil {
		return nil, err
	}

	duration := time.Since(start)
	gctx.Metrics().RecordHistogram(ctx, "app_gRPC-Server_stats", float64(duration.Milliseconds())+float64(duration.Nanoseconds()%1e6)/1e6, "gRPC_Service", "Hello", "method", "SayHello")

	resp, ok := res.(*HelloResponse)
	if !ok {
		return nil, status.Errorf(codes.Unknown, "unexpected response type %T", res)
	}

	return resp, nil
}

func (h *HelloServerWrapper) mustEmbedUnimplementedHelloServer() {}

func RegisterHelloServerWithGofr(app *gofr.App, srv HelloServerWithGofr) {
	var s grpc.ServiceRegistrar = app

	wrapper := &HelloServerWrapper{server: srv}

	gRPCBuckets := []float64{0.005, 0.01, .05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	app.Metrics().NewHistogram("app_gRPC-Server_stats", "Response time of gRPC server in milliseconds.", gRPCBuckets...)

	RegisterHelloServer(s, wrapper)
}

func (h *HelloServerWrapper) GetGofrContext(ctx context.Context, req gofr.Request) *gofr.Context {
	return &gofr.Context{
		Context:   ctx,
		Container: h.Container,
		Request:   req,
	}
}
type HelloRequestWrapper struct {
	ctx context.Context
	*HelloRequest
}

func (h *HelloRequestWrapper) Context() context.Context {
	return h.ctx
}

func (h *HelloRequestWrapper) Param(s string) string {
	return ""
}

func (h *HelloRequestWrapper) PathParam(s string) string {
	return ""
}

func (h *HelloRequestWrapper) Bind(p interface{}) error {
	ptr := reflect.ValueOf(p)
	if ptr.Kind() != reflect.Ptr {
		return fmt.Errorf("expected a pointer, got %T", p)
	}

	hValue := reflect.ValueOf(h.HelloRequest).Elem()
	ptrValue := ptr.Elem()

	// Ensure we can set exported fields (skip unexported fields)
	for i := 0; i < hValue.NumField(); i++ {
		field := hValue.Type().Field(i)
		// Skip the fields we don't want to copy (state, sizeCache, unknownFields)
		if field.Name == "state" || field.Name == "sizeCache" || field.Name == "unknownFields" {
			continue
		}

		if field.IsExported() {
			ptrValue.Field(i).Set(hValue.Field(i))
		}
	}

	return nil
}

func (h *HelloRequestWrapper) HostName() string {
	return ""
}

func (h *HelloRequestWrapper) Params(s string) []string {
	return nil
}
