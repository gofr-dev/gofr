package gofr

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/migration"
	"gofr.dev/pkg/gofr/service"
)

// App is the main application in the gofr framework.
type App struct {
	// Config can be used by applications to fetch custom configurations from environment or file.
	Config config.Config // If we directly embed, unnecessary confusion between app.Get and app.GET will happen.

	grpcServer   *grpcServer
	httpServer   *httpServer
	metricServer *metricServer

	cmd *cmd

	// container is unexported because this is an internal implementation and applications are provided access to it via Context
	container *container.Container

	grpcRegistered bool
	httpRegistered bool

	subscriptionManager SubscriptionManager
}

// RegisterService adds a grpc service to the gofr application.
func (a *App) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	a.container.Logger.Infof("Registering GRPC Server: %s", desc.ServiceName)
	a.grpcServer.server.RegisterService(desc, impl)
	a.grpcRegistered = true
}

// New creates an HTTP Server Application and returns that App.
func New() *App {
	app := &App{}
	app.readConfig()
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

	app.httpServer = newHTTPServer(app.container, port)

	// GRPC Server
	port, err = strconv.Atoi(app.Config.Get("GRPC_PORT"))
	if err != nil || port <= 0 {
		port = defaultGRPCPort
	}

	app.grpcServer = newGRPCServer(app.container, port)

	app.subscriptionManager = newSubscriptionManager(app.container)

	return app
}

// NewCMD creates a command line application.
func NewCMD() *App {
	app := &App{}
	app.readConfig()

	app.container = container.NewEmptyContainer()
	app.container.Logger = logging.NewFileLogger(app.Config.Get("CMD_LOGS_FILE"))
	app.cmd = &cmd{}

	app.container.Create(app.Config)
	app.initTracer()

	return app
}

// Run starts the application. If it is a HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.cmd.Run(a.container)
	}

	wg := sync.WaitGroup{}

	// Start Metrics Server
	// running metrics server before http and grpc
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
		a.httpServer.router.PathPrefix("/").Handler(handler{
			function:  catchAllHandler,
			container: a.container,
		})

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
	if a.subscriptionManager.subscriptions != nil {
		// Start subscribers concurrently using go-routines
		for topic, handler := range a.subscriptionManager.subscriptions {
			go a.subscriptionManager.startSubscriber(topic, handler)
		}

		wg.Add(1)
	}

	wg.Wait()
}

// readConfig reads the configuration from the default location.
func (a *App) readConfig() {
	var configLocation string
	if _, err := os.Stat("./configs"); err == nil {
		configLocation = "./configs"
	}

	a.Config = config.NewEnvFile(configLocation)
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

// GET adds a Handler for http GET method for a route pattern.
func (a *App) GET(pattern string, handler Handler) {
	a.add("GET", pattern, handler)
}

// PUT adds a Handler for http PUT method for a route pattern.
func (a *App) PUT(pattern string, handler Handler) {
	a.add("PUT", pattern, handler)
}

// POST adds a Handler for http POST method for a route pattern.
func (a *App) POST(pattern string, handler Handler) {
	a.add("POST", pattern, handler)
}

// DELETE adds a Handler for http DELETE method for a route pattern.
func (a *App) DELETE(pattern string, handler Handler) {
	a.add("DELETE", pattern, handler)
}

func (a *App) add(method, pattern string, h Handler) {
	a.httpRegistered = true
	a.httpServer.router.Add(method, pattern, handler{
		function:  h,
		container: a.container,
	})
}

func (a *App) Metrics() metrics.Manager {
	return a.container.Metrics()
}

// SubCommand adds a sub-command to the CLI application.
// Can be used to create commands like "kubectl get" or "kubectl get ingress".
func (a *App) SubCommand(pattern string, handler Handler) {
	a.cmd.addRoute(pattern, handler)
}

func (a *App) Migrate(migrationsMap map[int64]migration.Migrate) {
	migration.Run(migrationsMap, a.container)
}

func (a *App) initTracer() {
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.GetOrDefault("TRACER_PORT", "9411")

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(a.container.GetAppName()),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(&otelErrorHandler{logger: a.container.Logger})

	if tracerHost != "" {
		a.container.Log("Exporting traces to zipkin.")

		exporter, err := zipkin.New(
			fmt.Sprintf("http://%s:%s/api/v2/spans", tracerHost, tracerPort),
		)
		batcher := sdktrace.NewBatchSpanProcessor(exporter)
		tp.RegisterSpanProcessor(batcher)

		if err != nil {
			a.container.Error(err)
		}
	}
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

func (a *App) EnableBasicAuthWithFunc(validateFunc func(username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{ValidateFunc: validateFunc}))
}

func (a *App) EnableAPIKeyAuth(apiKeys ...string) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(nil, apiKeys...))
}

func (a *App) EnableAPIKeyAuthWithFunc(validator func(apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(validator))
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
		a.container.Logger.Errorf("Subscriber not initialized in the container")

		return
	}

	a.subscriptionManager.subscriptions[topic] = handler
}
