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
	"testing"
	"time"
)

// telemetryFlushTimeout bounds the metrics and traces flush/shutdown performed
// after a CMD app's handler returns, so a CLI invocation cannot hang
// indefinitely waiting on an unreachable collector.
const telemetryFlushTimeout = 10 * time.Second

// runCMD runs a CMD application's subcommand and then flushes telemetry: the
// final metric window and the pending span batch would otherwise be dropped when
// the process exits, which for a CLI invocation is every window and every batch.
// The flush is bounded by telemetryFlushTimeout so an unreachable collector
// cannot hang the invocation. If the command failed, it exits non-zero after the
// flush so shells and CI can detect the failure.
func (a *App) runCMD() {
	failed := a.cmd.Run(a.container)

	a.flushCMDTelemetry()

	if closer, ok := a.container.Logger.(io.Closer); ok {
		closer.Close()
	}

	// Exit non-zero (after telemetry is flushed and the logger is closed) so a failed
	// command is detectable by shells and CI. Skipped under `go test` so in-process
	// tests that invoke Run — including apps' own main() tests — are not terminated.
	if failed && !testing.Testing() {
		//nolint:revive // exit status 1 signals the failed command to shells and CI
		os.Exit(1)
	}
}

// flushCMDTelemetry flushes the final metric window and pending span batch after a
// CMD app's handler returns. Kept separate so its deferred cancel runs before
// runCMD's os.Exit on the failure path.
func (a *App) flushCMDTelemetry() {
	if a.container == nil {
		return
	}

	flushCtx, cancel := context.WithTimeout(context.Background(), telemetryFlushTimeout)
	defer cancel()

	if err := a.container.ShutdownMetrics(flushCtx); err != nil {
		a.Logger().Errorf("failed to flush metrics: %v", err)
	}

	if err := a.shutdownTraces(flushCtx); err != nil {
		a.Logger().Errorf("failed to flush traces: %v", err)
	}
}

// shutdownWaitMargin is the extra time Run gives the graceful-shutdown goroutine on top of the
// shutdown timeout that goroutine already bounds itself by. The wait is a safety net against a
// shutdown step that ignores its context, not a second deadline competing with the first one.
//
// One second because this value only ever decides how long a process hangs *after* it has already
// misbehaved, and both directions of error are bounded by that framing: too small and Run reports
// a shutdown that was about to finish as failed; too large and a pod that will never finish sits
// there until Kubernetes SIGKILLs it at terminationGracePeriodSeconds. A second is long enough to
// cover the scheduling and log-flush tail after the last shutdown step returns — the only work
// that legitimately happens past the deadline — and short enough to stay well inside the gap
// operators leave between SHUTDOWN_GRACE_PERIOD and terminationGracePeriodSeconds. It is
// deliberately not configurable: a knob here would be a second shutdown deadline to reason about,
// and SHUTDOWN_GRACE_PERIOD is the one that should move.
const shutdownWaitMargin = time.Second

// Run starts the application. If it is an HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.runCMD()
		return
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

	shutdownDone := a.startShutdownHandler(ctx, timeout)
	a.startTelemetryIfEnabled()
	a.startAllServers(ctx)
	a.awaitShutdown(ctx, shutdownDone, timeout)
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

// startShutdownHandler starts a goroutine to handle graceful shutdown. The returned channel is
// closed once that goroutine has finished, so Run can wait for it instead of letting the process
// exit while shutdown is still in flight.
func (a *App) startShutdownHandler(ctx context.Context, timeout time.Duration) <-chan struct{} {
	done := make(chan struct{})

	// Goroutine to handle shutdown when context is canceled
	go func() {
		defer close(done)

		<-ctx.Done()

		// Create a shutdown context with a timeout
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()

		if a.hasTelemetry() {
			a.sendTelemetry(http.DefaultClient, false)
		}

		a.Logger().Infof("Shutting down server with a timeout of %v", timeout)

		shutdownErr := a.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			a.Logger().Debugf("Server shutdown failed: %v", shutdownErr)
		}
	}()

	return done
}

// awaitShutdown blocks until the graceful shutdown started by startShutdownHandler has finished,
// so a main that only calls Run does not return — and let the process exit — while the datasources
// are still being closed.
//
// It returns at once when the servers stopped for a reason other than a termination signal: no
// shutdown is running in that case, and the handler goroutine is still parked on a context that is
// canceled only once Run returns.
func (a *App) awaitShutdown(ctx context.Context, done <-chan struct{}, timeout time.Duration) {
	if ctx.Err() == nil {
		return
	}

	select {
	case <-done:
	case <-time.After(timeout + shutdownWaitMargin):
		a.Logger().Errorf("graceful shutdown did not finish within %v, exiting anyway", timeout+shutdownWaitMargin)
	}
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
