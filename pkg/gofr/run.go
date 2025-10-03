package gofr

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Run starts the application. If it is an HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.cmd.Run(a.container)
	}

	// Create a context that is canceled on receiving termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !a.handleStartupHooks(ctx) {
		return
	}

	timeout, err := getShutdownTimeoutFromConfig(a.Config)
	if err != nil {
		a.Logger().Errorf("error parsing value of shutdown timeout from config: %v. Setting default timeout of 30 sec.", err)
	}

	a.startShutdownHandler(ctx, timeout)
	a.startTelemetryIfEnabled()
	a.startAllServers(ctx)
}

// handleStartupHooks runs the startup hooks and returns false if the application should exit.
func (a *App) handleStartupHooks(ctx context.Context) bool {
	if err := a.runOnStartHooks(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			a.Logger().Errorf("Startup failed: %v", err)

			return false
		}
		// If the error is context.Canceled, do not exit; allow graceful shutdown.
		a.Logger().Info("Startup canceled by context, shutting down gracefully.")

		return false
	}

	return true
}

// startShutdownHandler starts a goroutine to handle graceful shutdown.
func (a *App) startShutdownHandler(ctx context.Context, timeout time.Duration) {
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
}

// startTelemetryIfEnabled starts telemetry if it's enabled.
func (a *App) startTelemetryIfEnabled() {
	if a.hasTelemetry() {
		go a.sendTelemetry(http.DefaultClient, true)
	}
}

// startAllServers starts all registered servers concurrently.
func (a *App) startAllServers(ctx context.Context) {
	wg := sync.WaitGroup{}

	a.startMetricsServer(&wg)
	a.startHTTPServer(&wg)
	a.startGRPCServer(&wg)
	a.startSubscriptionManager(ctx, &wg)

	wg.Wait()
}

// startMetricsServer starts the metrics server if configured.
func (a *App) startMetricsServer(wg *sync.WaitGroup) {
	// Start Metrics Server
	// running metrics server before HTTP and gRPC
	if a.metricServer != nil {
		wg.Add(1)

		go func(m *metricServer) {
			defer wg.Done()

			m.Run(a.container)
		}(a.metricServer)
	}
}

// startHTTPServer starts the HTTP server if registered.
func (a *App) startHTTPServer(wg *sync.WaitGroup) {
	if a.httpRegistered {
		wg.Add(1)
		a.httpServerSetup()

		go func(s *httpServer) {
			defer wg.Done()

			s.run(a.container)
		}(a.httpServer)
	}
}

// startGRPCServer starts the gRPC server if registered.
func (a *App) startGRPCServer(wg *sync.WaitGroup) {
	if a.grpcRegistered {
		wg.Add(1)

		go func(s *grpcServer) {
			defer wg.Done()

			s.Run(a.container)
		}(a.grpcServer)
	}
}

// startSubscriptionManager starts the subscription manager.
func (a *App) startSubscriptionManager(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		err := a.startSubscriptions(ctx)
		if err != nil {
			a.Logger().Errorf("Subscription Error : %v", err)
		}
	}()
}
