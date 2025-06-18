package gofr

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strconv"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofr_grpc "gofr.dev/pkg/gofr/grpc"
)

type grpcServer struct {
	server             *grpc.Server
	interceptors       []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	options            []grpc.ServerOption
	port               int
	config             config.Config
}

// AddGRPCServerOptions allows users to add custom gRPC server options such as TLS configuration,
// timeouts, interceptors, and other server-specific settings in a single call.
//
// Example:
//
//	// Add TLS credentials and connection timeout in one call
//	creds, _ := credentials.NewServerTLSFromFile("server-cert.pem", "server-key.pem")
//	app.AddGRPCServerOptions(
//		grpc.Creds(creds),
//		grpc.ConnectionTimeout(10 * time.Second),
//	)
//
// This function accepts a variadic list of gRPC server options (grpc.ServerOption) and appends them
// to the server's configuration. It allows fine-tuning of the gRPC server's behavior during its initialization.
func (a *App) AddGRPCServerOptions(grpcOpts ...grpc.ServerOption) {
	a.grpcServer.options = append(a.grpcServer.options, grpcOpts...)
}

// AddGRPCUnaryInterceptors allows users to add custom gRPC interceptors.
// Example:
//
//	func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
//	handler grpc.UnaryHandler) (interface{}, error) {
//		log.Printf("Received gRPC request: %s", info.FullMethod)
//		return handler(ctx, req)
//	}
//	app.AddGRPCUnaryInterceptors(loggingInterceptor)
func (a *App) AddGRPCUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) {
	a.grpcServer.interceptors = append(a.grpcServer.interceptors, interceptors...)
}

func (a *App) AddGRPCServerStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) {
	a.grpcServer.streamInterceptors = append(a.grpcServer.streamInterceptors, interceptors...)
}

func newGRPCServer(c *container.Container, port int, cfg config.Config) *grpcServer {
	middleware := make([]grpc.UnaryServerInterceptor, 0)
	middleware = append(middleware,
		grpc_recovery.UnaryServerInterceptor(),
		gofr_grpc.ObservabilityInterceptor(c.Logger, c.Metrics()))

	streamMiddleware := make([]grpc.StreamServerInterceptor, 0)
	streamMiddleware = append(streamMiddleware,
		grpc_recovery.StreamServerInterceptor(),
		gofr_grpc.StreamObservabilityInterceptor(c.Logger, c.Metrics()))

	return &grpcServer{
		port:               port,
		interceptors:       middleware,
		streamInterceptors: streamMiddleware,
		config:             cfg,
	}
}

func (g *grpcServer) createServer() {
	interceptorOption := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(g.interceptors...))
	streamOpt := grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(g.streamInterceptors...))
	g.options = append(g.options, interceptorOption, streamOpt)

	g.server = grpc.NewServer(g.options...)

	enabled := strings.ToLower(g.config.GetOrDefault("GRPC_ENABLE_REFLECTION", "false"))
	if enabled != defaultReflection {
		reflection.Register(g.server)
	}
}

func (g *grpcServer) Run(c *container.Container) {
	if g.server == nil {
		g.createServer()
	}

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
		if g.server != nil {
			g.server.GracefulStop()
		}

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

	if !a.grpcRegistered {
		a.grpcServer.createServer()
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
