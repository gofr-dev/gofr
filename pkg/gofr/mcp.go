package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/ai/mcp"
	"gofr.dev/pkg/gofr/container"
)

// MCPOption configures EnableMCP.
type MCPOption func(*mcpConfig)

type mcpConfig struct {
	writeTools bool
	exclude    map[string]bool
	inputs     map[string]reflect.Type // "METHOD /path" -> request body type, for richer tool schemas
}

// WithInput declares the request body type T for the route method+path, so that when the route is
// exposed as an MCP tool its input schema describes the body's fields and types in addition to the
// path parameters. T is expected to be a struct (or pointer to one); other kinds add no body schema.
//
// It is an EnableMCP option rather than a route option so registering routes (app.POST, ...) keeps
// its original signature:
//
//	app.POST("/orders", createOrder)
//	app.EnableMCP(gofr.WithInput[CreateOrder]("POST", "/orders"))
func WithInput[T any](method, path string) MCPOption {
	return func(c *mcpConfig) { c.inputs[method+" "+path] = reflect.TypeFor[T]() }
}

// WithWriteTools also exposes write handlers (POST/PUT/PATCH/DELETE) as tools. By default only
// read-only handlers are exposed so an agent cannot mutate state it was not explicitly granted.
func WithWriteTools() MCPOption {
	return func(c *mcpConfig) { c.writeTools = true }
}

// WithExcludedRoutes drops the given route path templates (e.g. "/internal/{id}") from the tools.
func WithExcludedRoutes(paths ...string) MCPOption {
	return func(c *mcpConfig) {
		for _, p := range paths {
			c.exclude[p] = true
		}
	}
}

// EnableMCP exposes the app's registered HTTP handlers as agent-callable tools over an MCP server on
// its own port (MCP_PORT, default 8200; MCP_PORT=0 disables the server). Read-only handlers are
// exposed by default; pass WithWriteTools to also expose write handlers. The tools are also reachable
// in handlers via ctx.LLM().Tools() regardless of whether the server is enabled.
func (a *App) EnableMCP(opts ...MCPOption) {
	cfg := &mcpConfig{exclude: make(map[string]bool), inputs: make(map[string]reflect.Type)}
	for _, o := range opts {
		o(cfg)
	}

	// Registering the tools (in-process capability) is separate from serving them over MCP (transport).
	tools := a.registerTools(cfg)

	port, ok := a.mcpPort()
	if !ok {
		return
	}

	server := mcp.NewServer(tools,
		mcp.WithServerInfo(a.container.GetAppName(), a.container.GetAppVersion()))

	a.mcpServer = newMCPServer(port, server)
}

func (a *App) mcpPort() (int, bool) {
	portStr := a.Config.Get("MCP_PORT")
	if portStr == "0" {
		a.container.Logger.Logf("MCP server is disabled (MCP_PORT=0)")
		return 0, false
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		port = defaultMCPPort
	}

	if !isPortAvailable(port) {
		a.container.Logger.Fatalf("MCP port %d is blocked or unreachable", port)
	}

	return port, true
}

type mcpServer struct {
	port    int
	handler http.Handler
	srv     *http.Server
}

func newMCPServer(port int, handler http.Handler) *mcpServer {
	return &mcpServer{port: port, handler: handler}
}

func (m *mcpServer) Run(c *container.Container) {
	c.Logf("Starting MCP server on port: %d", m.port)

	// Bind to loopback: the MCP transport authenticates only by passing through per-handler auth,
	// so it must not become a second network-reachable ingress to the service's handlers.
	m.srv = &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", m.port),
		Handler:           m.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := m.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		c.Errorf("error while listening to MCP server, err: %v", err)
	}
}

func (m *mcpServer) Shutdown(ctx context.Context) error {
	if m.srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return m.srv.Shutdown(ctx)
	}, nil)
}
