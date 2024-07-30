package gofr

import (
	"context"
	"errors"
	"fmt"
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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/migration"
	"gofr.dev/pkg/gofr/service"
)

const (
	defaultPublicStaticDir = "static"
	shutDownTimeout        = 30 * time.Second
	gofrTraceExporter      = "gofr"
	gofrTracerURL          = "https://tracer.gofr.dev"
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

// RegisterService adds a gRPC service to the GoFr application.
func (a *App) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	a.container.Logger.Infof("registering GRPC Server: %s", desc.ServiceName)
	a.grpcServer.server.RegisterService(desc, impl)
	a.grpcRegistered = true
}

// New creates an HTTP Server Application and returns that App.
func New() *App {
	app := &App{}
	app.readConfig(false)
	app.container = container.NewContainer(app.Config)

	app.initTracer()

	// Metrics Server
	port, err := strconv.Atoi(app.Config.Get("METRICS_PORT"))
	if err != nil || port <= 0 {
		port = defaultMetricPort
	}

	app.metricServer = newMetricServer(port)

	// HTTP Server
	port, err = strconv.Atoi(app.Config.Get("HTTP_PORT"))
	if err != nil || port <= 0 {
		port = defaultHTTPPort
	}

	app.httpServer = newHTTPServer(app.container, port, middleware.GetConfigs(app.Config))

	if app.Config.Get("APP_ENV") == "DEBUG" {
		app.httpServer.RegisterProfilingRoutes()
	}

	// gRPC Server
	port, err = strconv.Atoi(app.Config.Get("GRPC_PORT"))
	if err != nil || port <= 0 {
		port = defaultGRPCPort
	}

	app.grpcServer = newGRPCServer(app.container, port)

	app.subscriptionManager = newSubscriptionManager(app.container)

	// static file server
	currentWd, _ := os.Getwd()
	checkDirectory := filepath.Join(currentWd, defaultPublicStaticDir)

	if _, err = os.Stat(checkDirectory); err == nil {
		app.AddStaticFiles(defaultPublicStaticDir, checkDirectory)
	}

	return app
}

// NewCMD creates a command-line application.
func NewCMD() *App {
	app := &App{}
	app.readConfig(true)
	app.container = container.NewContainer(nil)
	app.container.Logger = logging.NewFileLogger(app.Config.Get("CMD_LOGS_FILE"))
	app.cmd = &cmd{}
	app.container.Create(app.Config)
	app.initTracer()

	return app
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

		_ = a.Shutdown(shutdownCtx)
	}()

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

	// Start GRPC Server only if a service is registered
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

func (a *App) httpServerSetup() {
	// Add Default routes
	a.add(http.MethodGet, "/.well-known/health", healthHandler)
	a.add(http.MethodGet, "/.well-known/alive", liveHandler)
	a.add(http.MethodGet, "/favicon.ico", faviconHandler)

	if _, err := os.Stat("./static/openapi.json"); err == nil {
		a.add(http.MethodGet, "/.well-known/openapi.json", OpenAPIHandler)
		a.add(http.MethodGet, "/.well-known/swagger", SwaggerUIHandler)
		a.add(http.MethodGet, "/.well-known/{name}", SwaggerUIHandler)
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
	a.httpRegistered = true

	a.httpServer.router.Add(method, pattern, handler{
		function:       h,
		container:      a.container,
		requestTimeout: a.Config.Get("REQUEST_TIMEOUT"),
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

func (a *App) initTracer() {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(a.container.GetAppName()),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(&otelErrorHandler{logger: a.container.Logger})

	traceExporter := a.Config.Get("TRACE_EXPORTER")
	tracerURL := a.Config.Get("TRACER_URL")

	// deprecated : tracer_host and tracer_port are deprecated and will be removed in upcoming versions.
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.GetOrDefault("TRACER_PORT", "9411")

	if !isValidConfig(a.Logger(), traceExporter, tracerURL, tracerHost, tracerPort) {
		return
	}

	exporter, err := a.getExporter(traceExporter, tracerHost, tracerPort, tracerURL)

	if err != nil {
		a.container.Error(err)
	}

	batcher := sdktrace.NewBatchSpanProcessor(exporter)
	tp.RegisterSpanProcessor(batcher)
}

func isValidConfig(logger logging.Logger, name, url, host, port string) bool {
	if url != "" && name == "" {
		logger.Error("missing TRACE_EXPORTER config, should be provided with TRACER_URL to enable tracing")
		return false
	}

	//nolint:revive // early-return is not possible here, as below is the intentional logging flow
	if url == "" && name != "" && !strings.EqualFold(name, "gofr") {
		if host != "" && port != "" {
			logger.Warn("TRACER_HOST and TRACER_PORT are deprecated, use TRACER_URL instead")
		} else {
			logger.Error("missing TRACER_URL config, should be provided with TRACE_EXPORTER to enable tracing")
			return false
		}
	}

	return true
}

func (a *App) getExporter(name, host, port, url string) (sdktrace.SpanExporter, error) {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)

	authHeader := a.Config.Get("TRACER_AUTH_KEY")

	switch strings.ToLower(name) {
	case "otlp", "jaeger":
		exporter, err = buildOtlpExporter(a.Logger(), name, url, host, port, authHeader)
	case "zipkin":
		exporter, err = buildZipkinExporter(a.Logger(), url, host, port, authHeader)
	case gofrTraceExporter:
		exporter = buildGoFrExporter(a.Logger(), url)
	default:
		a.container.Errorf("unsupported TRACE_EXPORTER: %s", name)
	}

	return exporter, err
}

// buildOpenTelemetryProtocol using OpenTelemetryProtocol as the trace exporter
// jaeger accept OpenTelemetry Protocol (OTLP) over gRPC to upload trace data.
func buildOtlpExporter(logger logging.Logger, name, url, host, port, authHeader string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("%s:%s", host, port)
	}

	logger.Infof("Exporting traces to %s at %s", strings.ToLower(name), url)

	opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(url)}

	if authHeader != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{"Authorization": authHeader}))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}

func buildZipkinExporter(logger logging.Logger, url, host, port, authHeader string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("http://%s:%s/api/v2/spans", host, port)
	}

	logger.Infof("Exporting traces to zipkin at %s", url)

	var opts []zipkin.Option
	if authHeader != "" {
		opts = append(opts, zipkin.WithHeaders(map[string]string{"Authorization": authHeader}))
	}

	return zipkin.New(url, opts...)
}

func buildGoFrExporter(logger logging.Logger, url string) sdktrace.SpanExporter {
	if url == "" {
		url = "https://tracer-api.gofr.dev/api/spans"
	}

	logger.Infof("Exporting traces to GoFr at %s", gofrTracerURL)

	return NewExporter(url, logging.NewLogger(logging.INFO))
}

type otelErrorHandler struct {
	logger logging.Logger
}

func (o *otelErrorHandler) Handle(e error) {
	o.logger.Error(e.Error())
}

// EnableBasicAuth enables basic authentication for the application.
//
// It takes a variable number of credentials as alternating username and password strings.
// An error is logged if an odd number of arguments is provided.
func (a *App) EnableBasicAuth(credentials ...string) {
	if len(credentials)%2 != 0 {
		a.container.Error("Invalid number of arguments for EnableBasicAuth")
	}

	users := make(map[string]string)
	for i := 0; i < len(credentials); i += 2 {
		users[credentials[i]] = credentials[i+1]
	}

	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{Users: users}))
}

// Deprecated: EnableBasicAuthWithFunc is deprecated and will be removed in future releases, users must use
// EnableBasicAuthWithValidator as it has access to application datasources.
func (a *App) EnableBasicAuthWithFunc(validateFunc func(username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{ValidateFunc: validateFunc, Container: a.container}))
}

// EnableBasicAuthWithValidator enables basic authentication for the HTTP server with a custom validator.
//
// The provided `validateFunc` is invoked for each authentication attempt. It receives a container instance,
// username, and password. The function should return `true` if the credentials are valid, `false` otherwise.
func (a *App) EnableBasicAuthWithValidator(validateFunc func(c *container.Container, username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{
		ValidateFuncWithDatasources: validateFunc, Container: a.container}))
}

// EnableAPIKeyAuth enables API key authentication for the application.
//
// It requires at least one API key to be provided. The provided API keys will be used to authenticate requests.
func (a *App) EnableAPIKeyAuth(apiKeys ...string) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{}, apiKeys...))
}

// Deprecated: EnableAPIKeyAuthWithFunc is deprecated and will be removed in future releases, users must use
// EnableAPIKeyAuthWithValidator as it has access to application datasources.
func (a *App) EnableAPIKeyAuthWithFunc(validateFunc func(apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFunc: validateFunc,
		Container:    a.container,
	}))
}

// EnableAPIKeyAuthWithValidator enables API key authentication for the application with a custom validation function.
//
// The provided `validateFunc` is used to determine the validity of an API key. It receives the request container
// and the API key as arguments and should return `true` if the key is valid, `false` otherwise.
func (a *App) EnableAPIKeyAuthWithValidator(validateFunc func(c *container.Container, apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFuncWithDatasources: validateFunc,
		Container:                   a.container,
	}))
}

// EnableOAuth configures OAuth middleware for the application.
//
// It registers a new HTTP service for fetching JWKS and sets up OAuth middleware
// with the given JWKS endpoint and refresh interval.
//
// The JWKS endpoint is used to retrieve JSON Web Key Sets for verifying tokens.
// The refresh interval specifies how often to refresh the token cache.
func (a *App) EnableOAuth(jwksEndpoint string, refreshInterval int) {
	a.AddHTTPService("gofr_oauth", jwksEndpoint)

	oauthOption := middleware.OauthConfigs{
		Provider:        a.container.GetHTTPService("gofr_oauth"),
		RefreshInterval: time.Second * time.Duration(refreshInterval),
	}

	a.httpServer.router.Use(middleware.OAuth(middleware.NewOAuth(oauthOption)))
}

// Subscribe registers a handler for the given topic.
//
// If the subscriber is not initialized in the container, an error is logged and
// the subscription is not registered.
func (a *App) Subscribe(topic string, handler SubscribeFunc) {
	if a.container.GetSubscriber() == nil {
		a.container.Logger.Errorf("subscriber not initialized in the container")

		return
	}

	a.subscriptionManager.subscriptions[topic] = handler
}

// AddRESTHandlers creates and registers CRUD routes for the given struct, the struct should always be passed by reference.
func (a *App) AddRESTHandlers(object interface{}) error {
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

// AddCronJob registers a cron job to the cron table, the schedule is in * * * * * (5 part) format
// denoting minute, hour, day, month and day of week respectively.
func (a *App) AddCronJob(schedule, jobName string, job CronFunc) {
	if a.cron == nil {
		a.cron = NewCron(a.container)
	}

	if err := a.cron.AddJob(schedule, jobName, job); err != nil {
		a.Logger().Errorf("error adding cron job, err : %v", err)
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
	a.httpRegistered = true

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

	a.httpServer.router.AddStaticFiles(endpoint, filePath)
}
