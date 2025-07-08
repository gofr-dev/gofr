package gofr

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gofr.dev/pkg/gofr/logging"
)

// Run starts the application. If it is an HTTP server, it will start the server.
func (a *App) Run() {
	a.runStartupHooks()

	if a.cmd != nil {
		a.cmd.Run(a.container)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	timeout := a.getShutdownTimeout()
	a.handleShutdown(ctx, timeout)

	if a.hasTelemetry() {
		go a.sendTelemetry(http.DefaultClient, true)
	}

	a.startServers(ctx)
}

func (a *App) runStartupHooks() {
	sc := &StartupContext{
		Context:       context.Background(),
		Container:     a.container,
		ContextLogger: *logging.NewContextLogger(context.Background(), a.Logger()),
	}
	for _, hook := range a.startupHooks {
		if err := hook(sc); err != nil {
			a.Logger().Errorf("startup hook failed: %v", err)
			os.Exit(1)
		}
	}
}

func (a *App) getShutdownTimeout() (timeout time.Duration) {
	timeout, err := getShutdownTimeoutFromConfig(a.Config)
	if err != nil {
		a.Logger().Errorf("error parsing value of shutdown timeout from config: %v. Setting default timeout of 30 sec.", err)
	}
	return
}

func (a *App) handleShutdown(ctx context.Context, timeout time.Duration) {
	go func() {
		<-ctx.Done()
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

func (a *App) startServers(ctx context.Context) {
	wg := sync.WaitGroup{}

	if a.metricServer != nil {
		wg.Add(1)
		go func(m *metricServer) {
			defer wg.Done()
			m.Run(a.container)
		}(a.metricServer)
	}

	if a.httpRegistered {
		wg.Add(1)
		a.httpServerSetup()
		go func(s *httpServer) {
			defer wg.Done()
			s.run(a.container)
		}(a.httpServer)
	}

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
