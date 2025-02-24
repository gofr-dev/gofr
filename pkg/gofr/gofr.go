package gofr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/migration"
	"gofr.dev/pkg/gofr/service"
)

// App is the main application in the GoFr framework.
type App struct {
	// Config can be used by applications to fetch custom configurations from environment or file.
	Config config.Config // If we directly embed, unnecessary confusion between app.Get and app.GET will happen.

	grpcServer   *grpcServer
	httpServer   *httpServer
	metricServer *metricServer

	cmd  *cmd
	cron *Crontab

	// container is unexported because this is an internal implementation and applications are provided access to it via Context
	container *container.Container

	grpcRegistered bool
	httpRegistered bool

	subscriptionManager SubscriptionManager
}

// Run starts the application. If it is an HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.cmd.Run(a.container)
	}

	// Create a context that is canceled on receiving termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Goroutine to handle shutdown when context is canceled
	go func() {
		<-ctx.Done()

		// Create a shutdown context with a timeout
		shutdownCtx, done := context.WithTimeout(context.WithoutCancel(ctx), shutDownTimeout)
		defer done()

		if a.hasTelemetry() {
			a.sendTelemetry(http.DefaultClient, false)
		}

		_ = a.Shutdown(shutdownCtx)
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
			s.Run(a.container)
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

// Shutdown stops the service(s) and close the application.
// It shuts down the HTTP, gRPC, Metrics servers and closes the container's active connections to datasources.
func (a *App) Shutdown(ctx context.Context) error {
	var err error
	if a.httpServer != nil {
		err = errors.Join(err, a.httpServer.Shutdown(ctx))
	}

	if a.grpcServer != nil {
		err = errors.Join(err, a.grpcServer.Shutdown(ctx))
	}

	if a.container != nil {
		err = errors.Join(err, a.container.Close())
	}

	if a.metricServer != nil {
		err = errors.Join(err, a.metricServer.Shutdown(ctx))
	}

	if err != nil {
		a.container.Logger.Errorf("error while shutting down: %v", err)
		return err
	}

	a.container.Logger.Info("Application shutdown complete")

	return err
}

func isPortAvailable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf(":%d", port), checkPortTimeout)
	if err != nil {
		return true
	}

	conn.Close()

	return false
}

func (a *App) httpServerSetup() {
	// TODO: find a way to read REQUEST_TIMEOUT config only once and log it there. currently doing it twice one for populating
	// the value and other for logging
	requestTimeout := a.Config.Get("REQUEST_TIMEOUT")
	if requestTimeout != "" {
		timeoutVal, err := strconv.Atoi(requestTimeout)
		if err != nil || timeoutVal < 0 {
			a.container.Error("invalid value of config REQUEST_TIMEOUT.")
		}
	}

	a.httpServer.router.PathPrefix("/").Handler(handler{
		function:  catchAllHandler,
		container: a.container,
	})

	var registeredMethods []string

	_ = a.httpServer.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		met, _ := route.GetMethods()
		for _, method := range met {
			if !contains(registeredMethods, method) { // Check for uniqueness before adding
				registeredMethods = append(registeredMethods, method)
			}
		}

		return nil
	})

	*a.httpServer.router.RegisteredRoutes = registeredMethods
}

func (a *App) startSubscriptions(ctx context.Context) error {
	if len(a.subscriptionManager.subscriptions) == 0 {
		return nil
	}

	group := errgroup.Group{}
	// Start subscribers concurrently using go-routines
	for topic, handler := range a.subscriptionManager.subscriptions {
		subscriberTopic, subscriberHandler := topic, handler

		group.Go(func() error {
			return a.subscriptionManager.startSubscriber(ctx, subscriberTopic, subscriberHandler)
		})
	}

	return group.Wait()
}

// readConfig reads the configuration from the default location.
func (a *App) readConfig(isAppCMD bool) {
	var configLocation string
	if _, err := os.Stat("./configs"); err == nil {
		configLocation = "./configs"
	}

	if isAppCMD {
		a.Config = config.NewEnvFile(configLocation, logging.NewFileLogger(""))

		return
	}

	a.Config = config.NewEnvFile(configLocation, logging.NewLogger(logging.INFO))
}

// AddHTTPService registers HTTP service in container.
func (a *App) AddHTTPService(serviceName, serviceAddress string, options ...service.Options) {
	if a.container.Services == nil {
		a.container.Services = make(map[string]service.HTTP)
	}

	if _, ok := a.container.Services[serviceName]; ok {
		a.container.Debugf("Service already registered Name: %v", serviceName)
	}

	a.container.Services[serviceName] = service.NewHTTPService(serviceAddress, a.container.Logger, a.container.Metrics(), options...)
}

// GET adds a Handler for HTTP GET method for a route pattern.
func (a *App) GET(pattern string, handler Handler) {
	a.add("GET", pattern, handler)
}

// PUT adds a Handler for HTTP PUT method for a route pattern.
func (a *App) PUT(pattern string, handler Handler) {
	a.add("PUT", pattern, handler)
}

// POST adds a Handler for HTTP POST method for a route pattern.
func (a *App) POST(pattern string, handler Handler) {
	a.add("POST", pattern, handler)
}

// DELETE adds a Handler for HTTP DELETE method for a route pattern.
func (a *App) DELETE(pattern string, handler Handler) {
	a.add("DELETE", pattern, handler)
}

// PATCH adds a Handler for HTTP PATCH method for a route pattern.
func (a *App) PATCH(pattern string, handler Handler) {
	a.add("PATCH", pattern, handler)
}

func (a *App) add(method, pattern string, h Handler) {
	if !a.httpRegistered && !isPortAvailable(a.httpServer.port) {
		a.container.Logger.Fatalf("http port %d is blocked or unreachable", a.httpServer.port)
	}

	a.httpRegistered = true

	reqTimeout, err := strconv.Atoi(a.Config.Get("REQUEST_TIMEOUT"))
	if (err != nil && a.Config.Get("REQUEST_TIMEOUT") != "") || reqTimeout < 0 {
		reqTimeout = 0
	}

	a.httpServer.router.Add(method, pattern, handler{
		function:       h,
		container:      a.container,
		requestTimeout: time.Duration(reqTimeout) * time.Second,
	})
}

// Metrics returns the metrics manager associated with the App.
func (a *App) Metrics() metrics.Manager {
	return a.container.Metrics()
}

// Logger returns the logger instance associated with the App.
func (a *App) Logger() logging.Logger {
	return a.container.Logger
}

// SubCommand adds a sub-command to the CLI application.
// Can be used to create commands like "kubectl get" or "kubectl get ingress".
func (a *App) SubCommand(pattern string, handler Handler, options ...Options) {
	a.cmd.addRoute(pattern, handler, options...)
}

// Migrate applies a set of migrations to the application's database.
//
// The migrationsMap argument is a map where the key is the version number of the migration
// and the value is a migration.Migrate instance that implements the migration logic.
func (a *App) Migrate(migrationsMap map[int64]migration.Migrate) {
	// TODO : Move panic recovery at central location which will manage for all the different cases.
	defer func() {
		panicRecovery(recover(), a.container.Logger)
	}()

	migration.Run(migrationsMap, a.container)
}

// Subscribe registers a handler for the given topic.
//
// If the subscriber is not initialized in the container, an error is logged and
// the subscription is not registered.
func (a *App) Subscribe(topic string, handler SubscribeFunc) {
	if topic == "" || handler == nil {
		a.container.Logger.Errorf("invalid subscription: topic and handler must not be empty or nil")

		return
	}

	if a.container.GetSubscriber() == nil {
		a.container.Logger.Errorf("subscriber not initialized in the container")

		return
	}

	a.subscriptionManager.subscriptions[topic] = handler
}

// AddRESTHandlers creates and registers CRUD routes for the given struct, the struct should always be passed by reference.
func (a *App) AddRESTHandlers(object any) error {
	cfg, err := scanEntity(object)
	if err != nil {
		a.container.Logger.Errorf(err.Error())
		return err
	}

	a.registerCRUDHandlers(cfg, object)

	return nil
}

// UseMiddleware is a setter method for adding user defined custom middleware to GoFr's router.
func (a *App) UseMiddleware(middlewares ...gofrHTTP.Middleware) {
	a.httpServer.router.UseMiddleware(middlewares...)
}

// UseMiddlewareWithContainer adds a middleware that has access to the container
// and wraps the provided handler with the middleware logic.
//
// The `middleware` function receives the container and the handler, allowing
// the middleware to modify the request processing flow.
// Deprecated: UseMiddlewareWithContainer will be removed in a future release.
// Please use the [*App.UseMiddleware] method that does not depend on the container.
func (a *App) UseMiddlewareWithContainer(middlewareHandler func(c *container.Container, handler http.Handler) http.Handler) {
	a.httpServer.router.Use(func(h http.Handler) http.Handler {
		// Wrap the provided handler `h` with the middleware function `middlewareHandler`
		return middlewareHandler(a.container, h)
	})
}

// AddCronJob registers a cron job to the cron table.
// The cron expression can be either a 5-part or 6-part format. The 6-part format includes an
// optional second field (in beginning) and others being minute, hour, day, month and day of week respectively.
func (a *App) AddCronJob(schedule, jobName string, job CronFunc) {
	if a.cron == nil {
		a.cron = NewCron(a.container)
	}

	if err := a.cron.AddJob(schedule, jobName, job); err != nil {
		a.Logger().Errorf("error adding cron job, err: %v", err)
	}
}

// contains is a helper function checking for duplicate entry in a slice.
func contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}

	return false
}

// AddStaticFiles registers a static file endpoint for the application.
//
// The provided `endpoint` will be used as the prefix for the static file
// server. The `filePath` specifies the directory containing the static files.
// If `filePath` starts with "./", it will be interpreted as a relative path
// to the current working directory.
func (a *App) AddStaticFiles(endpoint, filePath string) {
	if !a.httpRegistered && !isPortAvailable(a.httpServer.port) {
		a.container.Logger.Fatalf("http port %d is blocked or unreachable", a.httpServer.port)
	}

	a.httpRegistered = true

	if !strings.HasPrefix(filePath, "./") && !filepath.IsAbs(filePath) {
		filePath = "./" + filePath
	}

	// update file path based on current directory if it starts with ./
	if strings.HasPrefix(filePath, "./") {
		currentWorkingDir, _ := os.Getwd()
		filePath = filepath.Join(currentWorkingDir, filePath)
	}

	endpoint = "/" + strings.TrimPrefix(endpoint, "/")

	if _, err := os.Stat(filePath); err != nil {
		a.container.Logger.Errorf("error in registering '%s' static endpoint, error: %v", endpoint, err)
		return
	}

	a.container.Logger.Infof("registered static files at endpoint '%s' from directory '%s'", endpoint, filePath)

	a.httpServer.router.AddStaticFiles(endpoint, filePath)
}
