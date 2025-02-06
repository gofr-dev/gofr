package gofr

import (
	"context"
	"errors"
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
				gofr_grpc.ObservabilityInterceptor(c.Logger, c.Metrics()),
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

var (
	errNonAddressable = errors.New("cannot inject container as it is not addressable or is fail")
)

// RegisterService adds a gRPC service to the GoFr application.
func (a *App) RegisterService(desc *grpc.ServiceDesc, impl any) {
	if !a.grpcRegistered && !isPortAvailable(a.grpcServer.port) {
		a.container.Logger.Fatalf("gRPC port %d is blocked or unreachable", a.grpcServer.port)
	}

	a.container.Logger.Infof("registering gRPC Server: %s", desc.ServiceName)
	a.grpcServer.server.RegisterService(desc, impl)

	err := injectContainer(impl, a.container)
	if err != nil {
		return
	}

	a.grpcRegistered = true
}

func injectContainer(impl any, c *container.Container) error {
	val := reflect.ValueOf(impl)

	// Note: returning nil for the cases where user does not want to inject the container altogether and
	// not to break any existing implementation for the users that are using gRPC server. If users are
	// expecting the container to be injected and are passing non-addressable server struct, we have the
	// DEBUG log for the same.
	if val.Kind() != reflect.Pointer {
		c.Logger.Debugf("cannot inject container into non-addressable implementation of `%s`, consider using pointer",
			val.Type().Name())

		return nil
	}

	val = val.Elem()
	tVal := val.Type()

	for i := 0; i < val.NumField(); i++ {
		f := tVal.Field(i)
		v := val.Field(i)

		if f.Type == reflect.TypeOf(c) {
			if !v.CanSet() {
				c.Logger.Error(errNonAddressable)
				return errNonAddressable
			}

			v.Set(reflect.ValueOf(c))

			// early return expecting only one container field necessary for one gRPC implementation
			return nil
		}

		if f.Type == reflect.TypeOf(*c) {
			if !v.CanSet() {
				c.Logger.Error(errNonAddressable)
				return errNonAddressable
			}

			v.Set(reflect.ValueOf(*c))

			// early return expecting only one container field necessary for one gRPC implementation
			return nil
		}
	}

	return nil
}
