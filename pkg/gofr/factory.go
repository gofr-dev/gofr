package gofr

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gofr.dev/pkg/gofr/cmd/terminal"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

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

	if !isPortAvailable(port) {
		app.container.Logger.Fatalf("metrics port %d is blocked or unreachable", port)
	}

	app.metricServer = newMetricServer(port)

	// HTTP Server
	port, err = strconv.Atoi(app.Config.Get("HTTP_PORT"))
	if err != nil || port <= 0 {
		port = defaultHTTPPort
	}

	app.httpServer = newHTTPServer(app.container, port, middleware.GetConfigs(app.Config))
	app.httpServer.certFile = app.Config.GetOrDefault("CERT_FILE", "")
	app.httpServer.keyFile = app.Config.GetOrDefault("KEY_FILE", "")
	app.httpServer.staticFiles = make(map[string]string)

	// Add Default routes
	app.add(http.MethodGet, service.HealthPath, healthHandler)
	app.add(http.MethodGet, service.AlivePath, liveHandler)
	app.add(http.MethodGet, "/favicon.ico", faviconHandler)

	app.checkAndAddOpenAPIDocumentation()

	// gRPC Server
	port, err = strconv.Atoi(app.Config.Get("GRPC_PORT"))
	if err != nil || port <= 0 {
		port = defaultGRPCPort
	}

	app.grpcServer, _ = newGRPCServer(app.container, port, app.Config)

	app.subscriptionManager = newSubscriptionManager(app.container)

	// static file server
	currentWd, _ := os.Getwd()
	checkDirectory := filepath.Join(currentWd, defaultPublicStaticDir)

	if _, err = os.Stat(checkDirectory); err == nil {
		app.httpServer.staticFiles[checkDirectory] = "/static"
	}

	return app
}

// NewCMD creates a command-line application.
func NewCMD() *App {
	app := &App{}
	app.readConfig(true)
	app.container = container.NewContainer(nil)
	app.container.Logger = logging.NewFileLogger(app.Config.Get("CMD_LOGS_FILE"))

	app.cmd = &cmd{
		out: terminal.New(),
	}

	app.container.Create(app.Config)
	app.initTracer()

	return app
}
