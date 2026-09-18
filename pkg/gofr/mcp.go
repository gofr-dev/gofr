package gofr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
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

// EnableMCP exposes the app's safe HTTP handlers — read-only methods (GET/HEAD/OPTIONS) and QUERY
// (RFC 10008, safe and idempotent, whose query payload is passed as a "body" tool argument) — as
// agent-callable tools over an MCP server on its own port (MCP_PORT, default 8200; MCP_PORT=0 disables
// the server). Write handlers (POST/PUT/PATCH/DELETE) are never exposed, so an agent cannot mutate
// state through this surface. The tools are also reachable in handlers via ctx.LLM().Tools() regardless
// of whether the server is enabled.
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

	port, enabled, err := a.mcpPort()
	if err != nil {
		// Recorded, not logged and not acted on: this runs in the application's own setup, where
		// there is no server to stop and no datasources to release yet. Run reports it, and aborts,
		// at the same point it reports a port it could not claim.
		a.mcpConfigErr = err

		return
	}

	if !enabled {
		return
	}

	server := mcp.NewServer(tools,
		mcp.WithServerInfo(a.container.GetAppName(), a.container.GetAppVersion()))

	a.mcpServer = newMCPServer(port, server)
}

// mcpPort resolves the port to serve MCP on from configuration. It reports enabled=false only when
// the server is switched off outright with MCP_PORT=0, and an error for a value that could never be
// served.
//
// That error is not softened into the default port. The same policy applies to a port that cannot
// be claimed: MCP was asked for, and a service that comes up silently lacking a transport it was
// configured to expose is the worse outcome. Folding to 8200 would apply the gentler policy to the
// less ambiguous mistake -- an occupied port can be a transient condition of the environment, while
// 99999 is wrong now and will be wrong on every restart -- and would land the service on the one
// port this package documents as colliding with Vault, having only logged an error.
//
// The caller records it rather than acting on it, because EnableMCP runs during the application's
// own setup where there is nothing to abort. Run reports it at the same point it reports a port it
// could not claim. See App.bindMCPServer.
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
func (a *App) mcpPort() (port int, enabled bool, err error) {
	portStr := strings.TrimSpace(a.Config.Get("MCP_PORT"))
	if portStr == "" {
		return defaultMCPPort, true, nil
	}

	// The configured value is deliberately not echoed back. It comes from Config.Get, which CodeQL
	// treats as a potentially sensitive source - a config store holds secrets as well as ports - and
	// a log line is the wrong place to reproduce one. The operator knows what they set; what they
	// need from this message is which variable was rejected and what happened instead.
	port, err = strconv.Atoi(portStr)
	if err != nil {
		return 0, false, errMCPPortNotANumber
	}

	// Comparing the parsed number rather than the raw string is what makes "00", "+0" and " 0 " mean
	// the same thing as "0". An operator who wrote one of those meant to switch the transport off;
	// the string compare this replaces fell through to the default instead and silently enabled it on
	// 8200 - the very port the default is documented to collide with.
	if port == 0 {
		// The literal, not portStr: the parsed value is known to be zero, and this avoids echoing
		// the raw config value for the reason given above.
		a.container.Logger.Logf("MCP server is disabled (MCP_PORT=0)")

		return 0, false, nil
	}

	if port < minTCPPort || port > maxTCPPort {
		return 0, false, fmt.Errorf("%w: %d is outside %d-%d", errMCPPortOutOfRange, port, minTCPPort, maxTCPPort)
	}

	return port, true, nil
}

type mcpServer struct {
	port    int
	handler http.Handler
	// srvMu guards srv, listener and stopped. Run writes srv on the serve goroutine; Shutdown reads
	// all three on the caller goroutine.
	srvMu sync.Mutex
	srv   *http.Server
	// listener is created by bind, before any server starts, and consumed by Run. Splitting the
	// bind from the serve is what lets a port conflict fail startup deterministically: see bind.
	listener net.Listener
	// stopped records a Shutdown that arrived before Run assigned srv, so Run does not serve.
	stopped bool
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
	// Check cancellation before listening rather than leaving it to ListenConfig. Listen only
	// observes the context while resolving a name, and the address here is a literal 127.0.0.1, so
	// there is no resolver step and a signal arriving in this window would otherwise go unnoticed -
	// the bind would succeed and startup would continue on a context that is already done. Measured:
	// Listen with an already-canceled context returns a live listener for "127.0.0.1:0" and
	// context.Canceled only for "localhost:0".
	if err := ctx.Err(); err != nil {
		return err
	}

	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", m.port))
	if err != nil {
		return err
	}

	m.srvMu.Lock()
	m.listener = l
	m.srvMu.Unlock()

	return nil
}

func (m *mcpServer) Run(c *container.Container) {
	// Assign under the lock, then serve on the local copy so the blocking Serve call never holds it
	// while Shutdown reads srv.
	srv := &http.Server{
		Handler:           m.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	m.srvMu.Lock()

	if m.stopped {
		m.srvMu.Unlock()
		c.Logf("MCP server was shut down before it started on port: %d", m.port)

		return
	}

	listener := m.listener
	if listener != nil {
		m.srv = srv
	}

	m.srvMu.Unlock()

	if listener == nil {
		c.Errorf("MCP server was not bound; refusing to serve on port %d", m.port)

		return
	}

	c.Logf("Starting MCP server on port: %d", m.port)

	// The port was already claimed by bind, so Serve cannot fail for being in use. Anything reported
	// here is a fault while already serving, which is the same class of event the HTTP server logs.
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		c.Errorf("error while serving the MCP server on port %d, err: %v", m.port, err)
	}
}

func (m *mcpServer) Shutdown(ctx context.Context) error {
	m.srvMu.Lock()
	m.stopped = true
	srv := m.srv

	// Run has not assigned srv yet, so there is no server for srv.Shutdown to stop — but bind may
	// already hold the port. Mark the server stopped so a later Run returns instead of serving, and
	// release the listener here, since nothing else would ever close it.
	if srv == nil {
		l := m.listener
		m.listener = nil
		m.srvMu.Unlock()

		if l == nil {
			return nil
		}

		return l.Close()
	}

	m.srvMu.Unlock()

	return ShutdownWithContext(ctx, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}, nil)
}
