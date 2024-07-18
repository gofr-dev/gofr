package gofr

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/container"
	grpc2 "gofr.dev/pkg/gofr/grpc"
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
				grpc2.LoggingInterceptor(c.Logger),
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

	go func() {
		if err := g.server.Serve(listener); err != nil {
			c.Logger.Errorf("error in starting gRPC server at %s: %s", addr, err)
		}
	}()

	// Handle system signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		s := <-sigCh
		c.Logger.Infof("received signal %v, shutting down gracefully", s)

		if err := g.Shutdown(context.Background()); err != nil {
			c.Logger.Errorf("error during grpc server shutdown: %v", err)
		}
	}()
}

func (g *grpcServer) Shutdown(ctx context.Context) error {
	stopCh := make(chan struct{})
	go func() {
		g.server.GracefulStop()
		close(stopCh)
	}()

	select {
	case <-ctx.Done():
		g.server.Stop() // Force stop if context is done
		return ctx.Err()
	case <-stopCh:
		return nil
	}
}
