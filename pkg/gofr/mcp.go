package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
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

// EnableMCP exposes the app's read-only HTTP handlers (GET/HEAD/OPTIONS) as agent-callable tools over
// an MCP server on its own port (MCP_PORT, default 8200; MCP_PORT=0 disables the server). Write
// handlers are never exposed, so an agent cannot mutate state through this surface. The tools are also
// reachable in handlers via ctx.LLM().Tools() regardless of whether the server is enabled.
//
// It performs no network I/O: the port is only resolved here, and bound later when the server runs.
// If it cannot be bound, the failure is logged and the application carries on without the MCP
// transport (see mcpServer.Run).
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

// mcpPort resolves the port to serve MCP on from configuration. It reports false only when the
// server is switched off outright with MCP_PORT=0.
//
// It deliberately does not check whether the port can be bound. The previous dial-based probe was
// redundant — ListenAndServe answers the same question authoritatively a moment later, and
// mcpServer.Run already reports its error — and answering it wrongly was expensive, because the
// response was Logger.Fatalf, i.e. os.Exit from library code. A dial reports whether something is
// currently listening, which is neither stable (the port can be taken between the probe and the
// Listen) nor the same question: a wildcard listener elsewhere on the port makes the dial succeed
// while binding 127.0.0.1 would still have worked, so a healthy service could be killed at startup.
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

	return port, true
}

type mcpServer struct {
	port    int
	handler http.Handler
	srvMu   sync.Mutex // guards srv, written by Run on the serve goroutine and read by Shutdown on the caller goroutine
	srv     *http.Server
}

func newMCPServer(port int, handler http.Handler) *mcpServer {
	return &mcpServer{port: port, handler: handler}
}

func (m *mcpServer) Run(c *container.Container) {
	c.Logf("Starting MCP server on port: %d", m.port)

	// Bind to loopback: the MCP transport authenticates only by passing through per-handler auth,
	// so it must not become a second network-reachable ingress to the service's handlers.
	// Assign under the lock, then serve on the local copy so the blocking
	// ListenAndServe call never holds it while Shutdown reads srv.
	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", m.port),
		Handler:           m.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	m.srvMu.Lock()
	m.srv = srv
	m.srvMu.Unlock()

	// A bind failure — most often the port already being in use — takes MCP down and nothing else.
	// The service's own HTTP surface is unaffected and the tools stay callable in-process through
	// ctx.LLM().Tools(), so this is reported and the application keeps serving rather than exiting:
	// an optional agent transport is not worth the whole process. The message names the port because
	// the error alone does not distinguish which of a service's listeners failed.
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		c.Errorf("MCP server on port %d is not serving, err: %v — the rest of the application is "+
			"unaffected and tools remain available in-process", m.port, err)
	}
}

func (m *mcpServer) Shutdown(ctx context.Context) error {
	m.srvMu.Lock()
	srv := m.srv
	m.srvMu.Unlock()

	if srv == nil {
		return nil
	}

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}, nil)
}
