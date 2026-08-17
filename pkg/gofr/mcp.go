package gofr

import (
	"context"
	"errors"
	"fmt"
	"net"
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
// It performs no network I/O: the port is only resolved here, and claimed later during Run. A port
// that cannot be claimed fails startup — see (*App).bindMCPServer — but it does so from Run, where
// the application can shut down cleanly, rather than from here.
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
// It deliberately does not check whether the port can be bound. The previous dial-based probe did,
// and answered with Logger.Fatalf — os.Exit from library code, during setup, with no chance to clean
// up and no way for a test to survive it. The probe was also the wrong instrument: a dial reports
// whether something is currently listening, which is neither stable (the port can be taken between
// the probe and the Listen) nor the same question, since a wildcard listener elsewhere on the port
// makes the dial succeed while binding 127.0.0.1 would still have worked.
//
// mcpServer.bind takes the port for real instead, and does it where a failure can abort the run
// properly.
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
	// listener is created by bind, before any server starts, and consumed by Run. Splitting the
	// bind from the serve is what lets a port conflict fail startup deterministically: see bind.
	listener net.Listener
}

func newMCPServer(port int, handler http.Handler) *mcpServer {
	return &mcpServer{port: port, handler: handler}
}

// bind claims the port and reports whether it could. It is called synchronously during startup,
// before any server goroutine is launched, and a failure aborts the run.
//
// Binding here rather than inside Run is what makes that abort safe. The servers are started as
// concurrent goroutines under one waitgroup, so a failure discovered inside Run would race the
// others: it could ask for shutdown before the HTTP server had assigned its own *http.Server,
// leaving Shutdown nothing to close and the process serving forever on a canceled context. Claiming
// the port up front means the decision is made while nothing is running and there is nothing to
// unwind.
//
// It also removes the question the old dial-based probe could only guess at. net.Listen does not
// report whether the port looks free, it takes it — so there is no window between the check and the
// claim, and no case where a wildcard listener elsewhere makes a bindable port look occupied.
//
// Bind to loopback: the MCP transport authenticates only by passing through per-handler auth, so it
// must not become a second network-reachable ingress to the service's handlers.
func (m *mcpServer) bind(ctx context.Context) error {
	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", m.port))
	if err != nil {
		return err
	}

	m.listener = l

	return nil
}

func (m *mcpServer) Run(c *container.Container) {
	if m.listener == nil {
		c.Errorf("MCP server was not bound; refusing to serve on port %d", m.port)

		return
	}

	c.Logf("Starting MCP server on port: %d", m.port)

	// Assign under the lock, then serve on the local copy so the blocking Serve call never holds it
	// while Shutdown reads srv.
	srv := &http.Server{
		Handler:           m.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	m.srvMu.Lock()
	m.srv = srv
	m.srvMu.Unlock()

	// The port was already claimed by bind, so Serve cannot fail for being in use. Anything reported
	// here is a fault while already serving, which is the same class of event the HTTP server logs.
	if err := srv.Serve(m.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		c.Errorf("error while serving the MCP server on port %d, err: %v", m.port, err)
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
