package gofr

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
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

	// static fileserver
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

	wg := sync.WaitGroup{}

	// Start Metrics Server
	// running metrics server before HTTP and gRPC
	wg.Add(1)

	go func(m *metricServer) {
		defer wg.Done()
		m.Run(a.container)
	}(a.metricServer)

	// Start HTTP Server
	if a.httpRegistered {
		wg.Add(1)

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

	// If subscriber is registered, block main go routine to wait for subscriber to receive messages
	if len(a.subscriptionManager.subscriptions) != 0 {
		// Start subscribers concurrently using go-routines
		for topic, handler := range a.subscriptionManager.subscriptions {
			go a.subscriptionManager.startSubscriber(topic, handler)
		}

		wg.Add(1)
	}

	wg.Wait()
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

func (a *App) Metrics() metrics.Manager {
	return a.container.Metrics()
}

func (a *App) Logger() logging.Logger {
	return a.container.Logger
}

// SubCommand adds a sub-command to the CLI application.
// Can be used to create commands like "kubectl get" or "kubectl get ingress".
func (a *App) SubCommand(pattern string, handler Handler, options ...Options) {
	a.cmd.addRoute(pattern, handler, options...)
}

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
	authHeader := a.Config.Get("TRACER_AUTH_KEY")

	// deprecated : tracer_host and tracer_port are deprecated and will be removed in upcoming versions.
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.Get("TRACER_PORT")

	if tracerURL == "" && (tracerHost != "" || tracerPort != "") {
		a.Logger().Warn("TRACER_HOST and TRACER_PORT are deprecated, use TRACER_URL instead")
	}

	exporter, err := a.getExporter(traceExporter, tracerHost, tracerPort, tracerURL, authHeader)
	if err != nil {
		a.container.Error(err)
	}

	batcher := sdktrace.NewBatchSpanProcessor(exporter)
	tp.RegisterSpanProcessor(batcher)
}

func (a *App) getExporter(name, host, port, url, authHeader string) (sdktrace.SpanExporter, error) {
	var (
		exporter sdktrace.SpanExporter
		ctx      context.Context
	)

	if name == "" {
		a.Logger().Errorf("missing TRACE_EXPORTER config, should be provided with TRACER_URL to enable tracing")
		return exporter, nil
	}

	if (host == "" && port == "") && url == "" && name != gofrTraceExporter {
		a.Logger().Errorf("missing TRACER_URL config, should be provided with TRACE_EXPORTER to enable tracing")
		return exporter, nil
	}

	switch strings.ToLower(name) {
	case "otlp":
		return a.buildOpenTelemetryProtocol(ctx, url, host, port, strings.ToLower(name), authHeader)
	case "jaeger":
		// jaeger accept OpenTelemetry Protocol (OTLP) .
		return a.buildOpenTelemetryProtocol(ctx, url, host, port, strings.ToLower(name), authHeader)
	case "zipkin":
		return a.buildZipkin(url, host, port, authHeader)
	case gofrTraceExporter:
		if url == "" {
			url = "https://tracer-api.gofr.dev/api/spans"
		}

		a.container.Logf("Exporting traces to GoFr at %s", gofrTracerURL)

		exporter = NewExporter(url, logging.NewLogger(logging.INFO))
	default:
		a.container.Errorf("unsupported TRACE_EXPORTER: %s", name)
		return exporter, nil
	}

	return exporter, nil
}

// buildOpenTelemetryProtocol using OpenTelemetryProtocol as the trace exporter
// jaeger accept OpenTelemetry Protocol (OTLP) over gRPC to upload trace data .
func (a *App) buildOpenTelemetryProtocol(ctx context.Context, url, host, port, exporter, authHeader string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("%s:%s", host, port)
	}

	a.container.Logf("Exporting traces to %s at %s", exporter, url)

	opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(url)}

	if authHeader != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{"Authorization": authHeader}))
	}

	return otlptracegrpc.New(ctx, opts...)
}

func (a *App) buildZipkin(url, host, port, authHeader string) (sdktrace.SpanExporter, error) {
	if url == "" {
		url = fmt.Sprintf("http://%s:%s/api/v2/spans", host, port)
	}

	a.container.Logf("Exporting traces to zipkin at %s", url)

	var opts []zipkin.Option

	if authHeader != "" {
		opts = append(opts, zipkin.WithHeaders(map[string]string{"Authorization": authHeader}))
	}

	return zipkin.New(url, opts...)
}

type otelErrorHandler struct {
	logger logging.Logger
}

func (o *otelErrorHandler) Handle(e error) {
	o.logger.Error(e.Error())
}

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

func (a *App) EnableBasicAuthWithValidator(validateFunc func(c *container.Container, username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{
		ValidateFuncWithDatasources: validateFunc, Container: a.container}))
}

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

func (a *App) EnableAPIKeyAuthWithValidator(validateFunc func(c *container.Container, apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFuncWithDatasources: validateFunc,
		Container:                   a.container,
	}))
}

func (a *App) EnableOAuth(jwksEndpoint string, refreshInterval int) {
	a.AddHTTPService("gofr_oauth", jwksEndpoint)

	oauthOption := middleware.OauthConfigs{
		Provider:        a.container.GetHTTPService("gofr_oauth"),
		RefreshInterval: time.Second * time.Duration(refreshInterval),
	}

	a.httpServer.router.Use(middleware.OAuth(middleware.NewOAuth(oauthOption)))
}

func (a *App) Subscribe(topic string, handler SubscribeFunc) {
	if a.container.GetSubscriber() == nil {
		a.container.Logger.Errorf("subscriber not initialized in the container")

		return
	}

	a.subscriptionManager.subscriptions[topic] = handler
}

func (a *App) AddRESTHandlers(object interface{}) error {
	cfg, err := scanEntity(object)
	if err != nil {
		a.container.Logger.Errorf("invalid object for AddRESTHandlers")

		return err
	}

	a.registerCRUDHandlers(cfg, object)

	return nil
}

// UseMiddleware is a setter method for adding user defined custom middleware to GoFr's router.
func (a *App) UseMiddleware(middlewares ...gofrHTTP.Middleware) {
	a.httpServer.router.UseMiddleware(middlewares...)
}

// AddCronJob registers a cron job to the cron table, the schedule is in * * * * * (6 part) format
// denoting minutes, hours, days, months and day of week respectively.
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
