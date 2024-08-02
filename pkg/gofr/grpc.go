package gofr

import (
	"context"
	"net"
	"reflect"
	"strconv"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/container"
	gofr_grpc "gofr.dev/pkg/gofr/grpc"
)

type grpcServer struct {
	server *grpc.Server
	port   int
}

func newGRPCServer(c *container.Container, port int) *grpcServer {
	return &grpcServer{
		server: grpc.NewServer(
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
				grpc_recovery.UnaryServerInterceptor(),
				gofr_grpc.LoggingInterceptor(c.Logger),
			))),
		port: port,
	}
}

func (g *grpcServer) Run(c *container.Container) {
	addr := ":" + strconv.Itoa(g.port)

	c.Logger.Infof("starting gRPC server at %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		c.Logger.Errorf("error in starting gRPC server at %s: %s", addr, err)
		return
	}

	if err := g.server.Serve(listener); err != nil {
		c.Logger.Errorf("error in starting gRPC server at %s: %s", addr, err)
		return
	}
}

func (g *grpcServer) Shutdown(ctx context.Context) error {
	return ShutdownWithContext(ctx, func(_ context.Context) error {
		g.server.GracefulStop()

		return nil
	}, func() error {
		g.server.Stop()

		return nil
	})
}

// RegisterService adds a gRPC service to the GoFr application.
func (a *App) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	a.container.Logger.Infof("registering GRPC Server: %s", desc.ServiceName)
	a.grpcServer.server.RegisterService(desc, impl)

	injectContainer(impl, a.container)

	a.grpcRegistered = true
}

func injectContainer(impl any, c *container.Container) {
	val := reflect.ValueOf(impl)

	if val.Kind() != reflect.Pointer {
		c.Logger.Debugf("cannot inject container into non-addressable implementation of `%s`, consider using pointer",
			val.Type().Name())
		return
	}

	val = val.Elem()
	tVal := val.Type()

	for i := 0; i < val.NumField(); i++ {
		f := tVal.Field(i)
		v := val.Field(i)

		if f.Type == reflect.TypeOf(c) {
			if !v.CanSet() {
				c.Logger.Errorf("cannot inject container as it is not addressable or is fail")
				continue
			}

			v.Set(reflect.ValueOf(c))
		}

		if f.Type == reflect.TypeOf(*c) {
			if !v.CanSet() {
				c.Logger.Errorf("cannot inject container as it is not addressable or is fail")
				continue
			}

			v.Set(reflect.ValueOf(*c))
		}
	}
}
