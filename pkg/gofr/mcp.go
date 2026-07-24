package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/ai/mcp"
	"gofr.dev/pkg/gofr/container"
)

// MCPOption configures EnableMCP.
type MCPOption func(*mcpConfig)

type mcpConfig struct {
	exclude map[string]bool
}

// WithExcludedRoutes drops the given route path templates (e.g. "/internal/{id}") from the tools.
func WithExcludedRoutes(paths ...string) MCPOption {
	return func(c *mcpConfig) {
		for _, p := range paths {
			c.exclude[p] = true
		}
	}
}

// EnableMCP exposes the app's safe HTTP handlers — read-only methods (GET/HEAD/OPTIONS) and QUERY
// (RFC 10008, safe and idempotent, whose query payload is passed as a "body" tool argument) — as
// agent-callable tools over an MCP server on its own port (MCP_PORT, default 8200; MCP_PORT=0 disables
// the server). Write handlers (POST/PUT/PATCH/DELETE) are never exposed, so an agent cannot mutate
// state through this surface. The tools are also reachable in handlers via ctx.LLM().Tools() regardless
// of whether the server is enabled.
func (a *App) EnableMCP(opts ...MCPOption) {
	cfg := &mcpConfig{exclude: make(map[string]bool)}
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
