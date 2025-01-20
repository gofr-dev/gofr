// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package server

import (
	"context"
	"fmt"
	"reflect"

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

func (h *HelloServerWrapper) mustEmbedUnimplementedHelloServer() {}

func RegisterHelloServerWithGofr(s grpc.ServiceRegistrar, srv HelloServerWithGofr) {
	wrapper := &HelloServerWrapper{server: srv}
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
