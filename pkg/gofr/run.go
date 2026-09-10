package gofr

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// metricsFlushTimeout bounds the metrics flush/shutdown performed after a CMD
// app's handler returns, so a CLI invocation cannot hang indefinitely waiting
// on an unreachable metrics collector.
const metricsFlushTimeout = 10 * time.Second

// Run starts the application. If it is an HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.cmd.Run(a.container)

		if a.container != nil {
			flushCtx, cancel := context.WithTimeout(context.Background(), metricsFlushTimeout)

			if err := a.container.ShutdownMetrics(flushCtx); err != nil {
				a.Logger().Errorf("failed to flush metrics: %v", err)
			}

			cancel()
		}

		if closer, ok := a.container.Logger.(io.Closer); ok {
			closer.Close()
		}

		return
	}

	// Create a context that is canceled on receiving termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !a.handleStartupHooks(ctx) {
		return
	}

	if !a.bindMCPServer(ctx) {
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
//
// A hook that fails abandons the run the same way an unclaimable MCP port does, and for the same
// reason has to release what startup has already opened: the container's datasources are live by
// the time the hooks run, and Run returns normally from here rather than exiting.
func (a *App) handleStartupHooks(ctx context.Context) bool {
	err := a.runOnStartHooks(ctx)
	if err == nil {
		return true
	}

	if errors.Is(err, context.Canceled) {
		// A canceled context is an operator stopping the process, not a broken hook.
		a.Logger().Info("Startup canceled by context, shutting down gracefully.")
	} else {
		a.Logger().Errorf("Startup failed: %v", err)
	}

	a.shutdownAfterFailedStartup()

	return false
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
	a.startMCPServer(&wg)
	a.startHTTPServer(&wg)
	a.startGRPCServer(&wg)
	a.startSubscriptionManager(ctx, &wg)

	wg.Wait()
}

// bindMCPServer claims the MCP port and reports whether startup may continue.
//
// It runs before any server goroutine is launched. A port that cannot be claimed is a startup
// failure: EnableMCP was called, so MCP was asked for, and a service that silently comes up without
// a transport it was configured to expose is worse than one that refuses to start. Returning false
// aborts Run the same way a failed OnStart hook does — no server has started, and no os.Exit is
// involved.
//
// Doing this here rather than inside mcpServer.Run is deliberate: the servers run as concurrent
// goroutines under a shared waitgroup, so a failure raised from inside one of them would race the
// others' startup rather than cleanly stopping it.
//
// Nothing is serving at this point, but the OnStart hooks have already run and the container's
// datasources are already open, so the abort releases them before returning rather than dropping
// them on the floor.
func (a *App) bindMCPServer(ctx context.Context) bool {
	if a.mcpServer == nil {
		return true
	}

	err := a.mcpServer.bind(ctx)
	if err == nil {
		return true
	}

	// ListenConfig.Listen honors cancellation, so a SIGINT or SIGTERM arriving inside the bind
	// window surfaces here as context.Canceled. That is an operator stopping the process, not a port
	// problem, and reporting the port remedy for it sends them looking for a conflict that does not
	// exist. handleStartupHooks draws the same distinction for the startup hooks.
	if errors.Is(err, context.Canceled) {
		a.Logger().Info("Startup canceled by context, shutting down gracefully.")
	} else {
		a.Logger().Errorf("MCP server cannot start on port %d: %v. Set MCP_PORT to a free port, or "+
			"MCP_PORT=0 to run without the MCP transport while keeping tools available in-process.",
			a.mcpServer.port, err)
	}

	a.shutdownAfterFailedStartup()

	return false
}

// shutdownAfterFailedStartup releases what startup has already opened when the run is abandoned
// before any server is up — a failed startup hook, or an MCP port that cannot be claimed. Run
// returns normally afterwards, so without this the datasource connections opened by the container
// would be left to process exit.
//
// The timeout is deliberately taken from a fresh Background context rather than the run's own: the
// run context may already be canceled (that is one of the ways startup is abandoned), and a
// shutdown that inherited it would be dead on arrival.
func (a *App) shutdownAfterFailedStartup() {
	// The error is ignored deliberately: getShutdownTimeoutFromConfig returns the default timeout
	// alongside it, and a malformed SHUTDOWN_GRACE_PERIOD has already been reported by the caller
	// that reaches Run's normal path.
	timeout, _ := getShutdownTimeoutFromConfig(a.Config)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := a.Shutdown(ctx); err != nil {
		a.Logger().Debugf("Shutdown after failed startup reported: %v", err)
	}
}

// startMCPServer starts the MCP server if app.EnableMCP was called.
func (a *App) startMCPServer(wg *sync.WaitGroup) {
	if a.mcpServer == nil {
		return
	}

	wg.Add(1)

	go func(m *mcpServer) {
		defer wg.Done()

		m.Run(a.container)
	}(a.mcpServer)
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
