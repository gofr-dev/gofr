package gofr

import (
	"os"
	"path/filepath"
	"strconv"

	"gofr.dev/pkg/gofr/ai/llm"
	"gofr.dev/pkg/gofr/cmd/terminal"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
)

// New creates an HTTP Server Application and returns that App.
func New() *App {
	app := &App{}
	app.readConfig(false)
	app.container = container.NewContainer(app.Config)

	app.initTracer()
	app.initMetricsServer()
	app.initLLM()

	// HTTP Server
	port, err := strconv.Atoi(app.Config.Get("HTTP_PORT"))
	if err != nil || port <= 0 {
		port = defaultHTTPPort
	}

	app.httpServer = newHTTPServer(app.container, port, middleware.GetConfigs(app.Config, app.container.Logger))
	app.httpServer.certFile = app.Config.GetOrDefault("CERT_FILE", "")
	app.httpServer.keyFile = app.Config.GetOrDefault("KEY_FILE", "")
	app.httpServer.staticFiles = make(map[string]string)

	// Note: Default routes (health, alive, favicon, swagger) are registered in httpServerSetup()
	// only when HTTP server actually starts. This prevents gRPC-only apps from starting HTTP server.

	// gRPC Server
	port, err = strconv.Atoi(app.Config.Get("GRPC_PORT"))
	if err != nil || port <= 0 {
		port = defaultGRPCPort
	}

	app.grpcServer, err = newGRPCServer(app.container, port, app.Config)

	// Continue without gRPC server rather than failing the entire app
	if err != nil {
		app.container.Logger.Errorf("failed to create gRPC server: %v", err)
	}

	app.subscriptionManager = newSubscriptionManager(app.container)

	// static file server
	currentWd, _ := os.Getwd()
	checkDirectory := filepath.Join(currentWd, defaultPublicStaticDir)

	if _, err = os.Stat(checkDirectory); err == nil {
		app.httpServer.staticFiles[checkDirectory] = "/static"
		app.httpRegistered = true
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

// initLLM registers an OpenAI-compatible LLM from configuration when LLM_PROVIDER is set, mirroring
// the zero-config wiring of Redis and SQL. The API key is read from <PROVIDER>_API_KEY. Call
// app.AddLLM to register a custom model or override this one.
func (a *App) initLLM() {
	provider := a.Config.Get("LLM_PROVIDER")
	if provider == "" {
		return
	}

	model := a.Config.Get("LLM_MODEL")
	if model == "" {
		a.container.Logger.Errorf("LLM_PROVIDER is set but LLM_MODEL is empty; skipping LLM setup")
		return
	}

	a.AddLLM(&llm.Client{
		Provider: llm.Provider(provider),
		Model:    model,
		BaseURL:  a.Config.Get("LLM_BASE_URL"),
	})
}

// initMetricsServer initializes the metrics server based on configuration.
// If METRICS_PORT is explicitly set to 0, the metrics server is disabled.
func (a *App) initMetricsServer() {
	metricsPortStr := a.Config.Get("METRICS_PORT")

	if metricsPortStr == "0" {
		a.container.Logger.Logf("Metrics server is disabled (METRICS_PORT=0)")
		return
	}

	port, err := strconv.Atoi(metricsPortStr)
	if err != nil || port <= 0 {
		port = defaultMetricPort
	}

	if !isPortAvailable(port) {
		a.container.Logger.Fatalf("metrics port %d is blocked or unreachable", port)
	}

	a.metricServer = newMetricServer(port)
}
