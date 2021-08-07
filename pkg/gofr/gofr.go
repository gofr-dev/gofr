package gofr

import (
	"fmt"
	"github.com/vikash/gofr/pkg/gofr/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"log"
	"os"
	"strconv"
	"sync"

	gofrHTTP "github.com/vikash/gofr/pkg/gofr/http"
)

// App is the main application in the gofr framework.
type App struct {
	httpServer *httpServer
	cmd        *cmd

	// container is unexported because this is an internal implementation and applications are provided access to it via Context
	container *Container

	// Config can be used by applications to fetch custom configurations from environment or file.
	Config Config // If we directly embed, unnecessary confusion between app.Get and app.GET will happen.
}

// New creates a HTTP Server Application and returns that App.
func New() *App {
	app := &App{}
	app.readConfig()
	app.container = newContainer(app.Config)

	app.initTracer()
	// HTTP Server
	port, err := strconv.Atoi(app.Config.Get("HTTP_PORT"))
	if err != nil || port <= 0 {
		port = defaultHTTPPort
	}

	app.httpServer = &httpServer{
		router: gofrHTTP.NewRouter(),
		port:   port,
	}

	return app
}

// NewCMD creates a command line application.
func NewCMD() *App {
	app := &App{}
	app.readConfig()

	app.container = newContainer(app.Config)
	app.cmd = &cmd{}

	app.initTracer()

	return app
}

// Run starts the application. If it is a HTTP server, it will start the server.
func (a *App) Run() {
	if a.cmd != nil {
		a.cmd.Run(a.container)
	}

	wg := sync.WaitGroup{}

	// Start HTTP Server
	if a.httpServer != nil {
		wg.Add(1)

		go func(s *httpServer) {
			defer wg.Done()
			s.Run(a.container)
		}(a.httpServer)
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
	a.httpServer.router.Add(method, pattern, handler{
		function:  h,
		container: a.container,
	})
}

// SubCommand adds a sub-command to the CLI application.
// Can be used to create commands like "kubectl get" or "kubectl get ingress".
func (a *App) SubCommand(pattern string, handler Handler) {
	a.cmd.addRoute(pattern, handler)
}

func (a *App) initTracer() {
	tracerHost := a.Config.Get("TRACER_HOST")
	tracerPort := a.Config.GetOrDefault("TRACER_PORT", "9411")
	if tracerHost == "" {
		return
	}

	a.container.Log("Exporting traces to zipkin.")

	exporter, err := zipkin.New(
		fmt.Sprintf("http://%s:%s/api/v2/spans", tracerHost, tracerPort),
		zipkin.WithSDKOptions(sdktrace.WithSampler(sdktrace.AlwaysSample())),
	)

	batcher := sdktrace.NewBatchSpanProcessor(exporter)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(batcher),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(a.Config.GetOrDefault("APP_NAME", "gofr-service")),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if err != nil {
		log.Fatal(err)
	}
}
