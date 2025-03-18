package gofr

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Run starts the application. If it is an HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.cmd.Run(a.container)
	}

	// Create a context that is canceled on receiving termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	timeout, err := getShutdownTimeoutFromConfig(a.Config)
	if err != nil {
		a.Logger().Errorf("error parsing value of shutdown timeout from config: %v. Setting default timeout of 30 sec.", err)
	}

	// Goroutine to handle shutdown when context is canceled
	go func() {
		<-ctx.Done()

		// Create a shutdown context with a timeout
		shutdownCtx, done := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer done()

		if a.hasTelemetry() {
			a.sendTelemetry(http.DefaultClient, false)
		}

		a.Logger().Infof("Shutting down server with a timeout of %v", timeout)

		shutdownErr := a.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			a.Logger().Debugf("Server shutdown failed: %v", shutdownErr)
		}
	}()

	if a.hasTelemetry() {
		go a.sendTelemetry(http.DefaultClient, true)
	}

	wg := sync.WaitGroup{}

	// Start Metrics Server
	// running metrics server before HTTP and gRPC
	if a.metricServer != nil {
		wg.Add(1)

		go func(m *metricServer) {
			defer wg.Done()
			m.Run(a.container)
		}(a.metricServer)
	}

	// Start HTTP Server
	if a.httpRegistered {
		wg.Add(1)
		a.httpServerSetup()

		go func(s *httpServer) {
			defer wg.Done()
			s.run(a.container)
		}(a.httpServer)
	}

	// Start gRPC Server only if a service is registered
	if a.grpcRegistered {
		wg.Add(1)

		go func(s *grpcServer) {
			defer wg.Done()
			s.Run(a.container)
		}(a.grpcServer)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		err := a.startSubscriptions(ctx)
		if err != nil {
			a.Logger().Errorf("Subscription Error : %v", err)
		}
	}()

	wg.Wait()
}
