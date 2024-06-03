package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"gofr.dev/examples/grpc-server/grpc"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	grpc.RegisterHelloServer(app, grpc.Server{})

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go app.Run()

	<-signalCtx.Done()
	app.Logger().Info("shutting down")
	app.Shutdown(context.Background())
}
