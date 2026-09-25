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

// telemetryFlushTimeout bounds the metrics and traces flush/shutdown performed
// after a CMD app's handler returns, so a CLI invocation cannot hang
// indefinitely waiting on an unreachable collector.
const telemetryFlushTimeout = 10 * time.Second

// exitCodeCommandFailed is what a CMD app reports to the process when its subcommand failed: the
// handler returned an error, or the subcommand was missing or unknown. Shells and CI read the exit
// status and nothing else, so a failed command that exited 0 would be recorded as a success.
const exitCodeCommandFailed = 1

// runCMD runs a CMD application's subcommand and then flushes telemetry: the
// final metric window and the pending span batch would otherwise be dropped when
// the process exits, which for a CLI invocation is every window and every batch.
// The flush is bounded by telemetryFlushTimeout so an unreachable collector
// cannot hang the invocation.
//
// If the command failed, runCMD then exits the process with exitCodeCommandFailed,
// after the flush and after the logger is closed. Because that is os.Exit, functions
// deferred in the application's main() do not run on the failure path; cleanup that
// must happen either way should not rely on a defer in main().
func (a *App) runCMD() {
	failed := a.cmd.Run(a.container)

	a.flushCMDTelemetry()

	if closer, ok := a.container.Logger.(io.Closer); ok {
		closer.Close()
	}

	if failed {
		a.exitProcess(exitCodeCommandFailed)
	}
}

// exitProcess reports code to the process through a.exit, or os.Exit when it is nil.
func (a *App) exitProcess(code int) {
	if a.exit != nil {
		a.exit(code)

		return
	}

	// deep-exit is the right rule and this is the exception it exists to make you argue for: runCMD
	// is the last thing a CMD app's main does (app.Run), telemetry has been flushed and the logger
	// closed, and the exit status is the only channel a shell or CI job reads to learn the command
	// failed. The logger's Fatal carries the same exemption for the same kind of reason.
	//nolint:revive // deep-exit: see above -- top of the call stack, after cleanup, nothing skipped.
	os.Exit(code)
}

// flushCMDTelemetry flushes the final metric window and pending span batch after a
// CMD app's handler returns. Kept separate so its deferred cancel runs before
// runCMD exits the process on the failure path.
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

	// Both steps have already released what startup opened by the time they report anything but
	// startupOK. What is left is deciding what the abandonment means to the process, which happens
	// in one place rather than inside each step. See finishAbandonedStartup.
	if outcome := a.handleStartupHooks(ctx); outcome != startupOK {
		a.finishAbandonedStartup(outcome)

		return
	}

	if outcome := a.bindMCPServer(ctx); outcome != startupOK {
		a.finishAbandonedStartup(outcome)

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

// startupOutcome is what a pre-server startup step reports back to Run.
//
// The distinction that matters is the second one from the third: both abandon the run, but only one
// of them is a failure. An operator sending SIGTERM during startup got exactly what they asked for,
// and reporting that as a failed start would have every orchestrator retrying a deliberate stop.
type startupOutcome int

const (
	// startupOK means the step succeeded and Run may continue.
	startupOK startupOutcome = iota
	// startupFailed means the run was abandoned because something was wrong. The process reports a
	// non-zero status.
	startupFailed
	// startupCanceled means the run was abandoned because the operator stopped it. The process
	// reports success, as it does for a signal received at any other time.
	startupCanceled
)

// finishAbandonedStartup reports an abandoned startup to the process.
//
// Everything the run opened has already been released by the step that abandoned it, so this only
// decides the exit status.
func (a *App) finishAbandonedStartup(outcome startupOutcome) {
	if outcome == startupFailed {
		a.abortStartup()
	}
}

// handleStartupHooks runs the startup hooks and reports whether Run may continue.
//
// A hook that fails abandons the run the same way an unclaimable MCP port does, and for the same
// reason has to release what startup has already opened: the container's datasources are live by
// the time the hooks run, and Run unwinds from here rather than exiting from inside the hook.
func (a *App) handleStartupHooks(ctx context.Context) startupOutcome {
	err := a.runOnStartHooks(ctx)
	if err == nil {
		return startupOK
	}

	outcome := startupFailed

	if errors.Is(err, context.Canceled) {
		// A canceled context is an operator stopping the process, not a broken hook.
		a.Logger().Info("Startup canceled by context, shutting down gracefully.")

		outcome = startupCanceled
	} else {
		a.Logger().Errorf("Startup failed: %v", err)
	}

	a.shutdownAfterFailedStartup()

	return outcome
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
func (a *App) bindMCPServer(ctx context.Context) startupOutcome {
	// An MCP_PORT that could never be served is reported here rather than at the point it was
	// parsed, because EnableMCP runs inside the application's own setup where there is nothing to
	// abort yet. It aborts for the same reason an occupied port does -- MCP was asked for and cannot
	// be provided -- and with more justification: an occupied port can be a transient condition of
	// the environment, while a value outside 1-65535 is unambiguously wrong and will be just as
	// wrong on the next start.
	if a.mcpConfigErr != nil {
		a.Logger().Errorf("MCP server cannot start: %v. Set MCP_PORT to a valid, free port, or "+
			"MCP_PORT=0 to run without the MCP transport while keeping tools available in-process.",
			a.mcpConfigErr)

		a.shutdownAfterFailedStartup()

		return startupFailed
	}

	if a.mcpServer == nil {
		return startupOK
	}

	err := a.mcpServer.bind(ctx)
	if err == nil {
		return startupOK
	}

	outcome := startupFailed

	// ListenConfig.Listen honors cancellation, so a SIGINT or SIGTERM arriving inside the bind
	// window surfaces here as context.Canceled. That is an operator stopping the process, not a port
	// problem, and reporting the port remedy for it sends them looking for a conflict that does not
	// exist. handleStartupHooks draws the same distinction for the startup hooks.
	if errors.Is(err, context.Canceled) {
		a.Logger().Info("Startup canceled by context, shutting down gracefully.")

		outcome = startupCanceled
	} else {
		a.Logger().Errorf("MCP server cannot start on port %d: %v. Set MCP_PORT to a free port, or "+
			"MCP_PORT=0 to run without the MCP transport while keeping tools available in-process.",
			a.mcpServer.port, err)
	}

	a.shutdownAfterFailedStartup()

	return outcome
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
	// Reported here rather than assumed to have been reported already. An earlier revision skipped
	// it on the grounds that Run's normal path logs it -- but that is exactly the path an abandoned
	// startup never reaches, so a malformed SHUTDOWN_GRACE_PERIOD went unmentioned in the only
	// situation where this function runs. The default returned alongside the error is still used: a
	// bad grace period must not stop the cleanup.
	timeout, err := getShutdownTimeoutFromConfig(a.Config)
	if err != nil {
		a.Logger().Errorf("invalid SHUTDOWN_GRACE_PERIOD, using %s to shut down after a failed startup: %v", timeout, err)
	}

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
