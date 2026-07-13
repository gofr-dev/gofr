package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/mcp"
	"gofr.dev/pkg/gofr/container"
)

var errToolStatus = errors.New("tool returned an error status")

const (
	headerAuthorization = "Authorization"
	headerXAPIKey       = "X-Api-Key" //nolint:gosec // G101: HTTP header name, not a credential
	schemaKeyType       = "type"
	schemaTypeString    = "string"
	schemaTypeObject    = "object"

	maxToolResponseBytes = 4 << 20 // cap a captured tool response at 4 MiB
)

// MCPOption configures EnableMCP.
type MCPOption func(*mcpConfig)

type mcpConfig struct {
	writeTools bool
	exclude    map[string]bool
}

// WithWriteTools also exposes write handlers (POST/PUT/PATCH/DELETE) as tools. By default only
// read-only handlers are exposed so an agent cannot mutate state it was not explicitly granted.
func WithWriteTools() MCPOption {
	return func(c *mcpConfig) { c.writeTools = true }
}

// MCPExclude drops the given route path templates (e.g. "/internal/{id}") from the exposed tools.
func MCPExclude(paths ...string) MCPOption {
	return func(c *mcpConfig) {
		for _, p := range paths {
			c.exclude[p] = true
		}
	}
}

// EnableMCP exposes the app's registered HTTP handlers as agent-callable tools over an MCP server
// on its own port (MCP_PORT, default 8200; MCP_PORT=0 disables it). Read-only handlers are exposed
// by default; pass WithWriteTools to also expose write handlers. The same tools are reachable in
// handlers via ctx.LLM().Tools().
func (a *App) EnableMCP(opts ...MCPOption) {
	cfg := &mcpConfig{exclude: make(map[string]bool)}
	for _, o := range opts {
		o(cfg)
	}

	tools := &routerTools{app: a, cfg: cfg}

	// The tools are always reachable in-process via ctx.LLM().Tools(); MCP_PORT=0 only disables the
	// external MCP server.
	a.container.SetTools(tools)

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

// routerTools exposes the app's registered routes as ai.Tools by walking the router for discovery
// and dispatching a synthesized request back through the router for invocation, so binding,
// validation and auth middleware all run exactly as for a real HTTP call.
type routerTools struct {
	app *App
	cfg *mcpConfig
}

func (rt *routerTools) List() []ai.ToolSpec {
	var specs []ai.ToolSpec

	_ = rt.app.httpServer.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}

		methods, err := route.GetMethods()
		if err != nil {
			return nil
		}

		for _, method := range methods {
			if spec, ok := rt.specFor(method, pathTemplate); ok {
				specs = append(specs, spec)
			}
		}

		return nil
	})

	return specs
}

func (rt *routerTools) Only(names ...string) ai.Tools {
	keep := make(map[string]bool, len(names))
	for _, n := range names {
		keep[n] = true
	}

	return &filteredTools{tools: rt, keep: keep}
}

func (rt *routerTools) Call(ctx context.Context, name string, args json.RawMessage) (ai.Result, error) {
	method, pathTemplate, ok := rt.route(name)
	if !ok {
		return ai.Result{}, ai.ErrToolNotFound
	}

	req, err := buildToolRequest(ctx, method, pathTemplate, args)
	if err != nil {
		return ai.Result{}, err
	}

	if h := mcp.HeadersFromContext(ctx); h != nil {
		copyAuthHeaders(req.Header, h)
	}

	rec := &responseCapture{header: make(http.Header), status: http.StatusOK}
	rt.app.httpServer.router.ServeHTTP(rec, req)

	if rec.status >= http.StatusBadRequest {
		return ai.Result{}, fmt.Errorf("%w: tool %q returned status %d", errToolStatus, name, rec.status)
	}

	return ai.NewResult(json.RawMessage(rec.body.Bytes())), nil
}

func (rt *routerTools) specFor(method, pathTemplate string) (ai.ToolSpec, bool) {
	if rt.cfg.exclude[pathTemplate] || strings.HasPrefix(pathTemplate, "/.well-known") {
		return ai.ToolSpec{}, false
	}

	access := accessForMethod(method)
	if access == ai.Write && !rt.cfg.writeTools {
		return ai.ToolSpec{}, false
	}

	return ai.ToolSpec{
		Name:        toolName(method, pathTemplate),
		Description: method + " " + pathTemplate,
		InputSchema: pathParamSchema(pathTemplate),
		Access:      access,
	}, true
}

func (rt *routerTools) route(name string) (method, pathTemplate string, found bool) {
	_ = rt.app.httpServer.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		pathT, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}

		methods, err := route.GetMethods()
		if err != nil {
			return nil
		}

		for _, m := range methods {
			if spec, ok := rt.specFor(m, pathT); ok && spec.Name == name {
				method, pathTemplate, found = m, pathT, true
			}
		}

		return nil
	})

	return method, pathTemplate, found
}

// filteredTools restricts a routerTools to a whitelist of tool names.
type filteredTools struct {
	tools *routerTools
	keep  map[string]bool
}

func (f *filteredTools) List() []ai.ToolSpec {
	var out []ai.ToolSpec

	for _, spec := range f.tools.List() {
		if f.keep[spec.Name] {
			out = append(out, spec)
		}
	}

	return out
}

func (f *filteredTools) Only(names ...string) ai.Tools {
	keep := make(map[string]bool)

	for _, n := range names {
		if f.keep[n] { // intersect with the current whitelist so chaining narrows, never widens
			keep[n] = true
		}
	}

	return &filteredTools{tools: f.tools, keep: keep}
}

func (f *filteredTools) Call(ctx context.Context, name string, args json.RawMessage) (ai.Result, error) {
	if !f.keep[name] {
		return ai.Result{}, ai.ErrToolNotFound
	}

	return f.tools.Call(ctx, name, args)
}

func accessForMethod(method string) ai.Access {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return ai.ReadOnly
	default:
		return ai.Write
	}
}

func toolName(method, pathTemplate string) string {
	var b strings.Builder

	b.WriteString(strings.ToLower(method))

	for _, seg := range strings.Split(pathTemplate, "/") {
		if seg == "" {
			continue
		}

		b.WriteByte('_')
		b.WriteString(paramName(seg))
	}

	return b.String()
}

func pathParamSchema(pathTemplate string) json.RawMessage {
	params := pathParams(pathTemplate)
	if len(params) == 0 {
		return nil
	}

	props := make(map[string]any, len(params))
	for _, p := range params {
		props[p] = map[string]string{schemaKeyType: schemaTypeString}
	}

	schema, _ := json.Marshal(map[string]any{
		schemaKeyType: schemaTypeObject,
		"properties":  props,
		"required":    params,
	})

	return schema
}

func pathParams(pathTemplate string) []string {
	var out []string

	for _, seg := range strings.Split(pathTemplate, "/") {
		if isParamSegment(seg) {
			out = append(out, paramName(seg))
		}
	}

	return out
}

func isParamSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

// paramName strips the braces and any mux regex constraint, e.g. "{id:[0-9]+}" -> "id".
func paramName(seg string) string {
	if !isParamSegment(seg) {
		return seg
	}

	inner := seg[1 : len(seg)-1]
	if i := strings.IndexByte(inner, ':'); i >= 0 {
		inner = inner[:i]
	}

	return inner
}

func buildToolRequest(ctx context.Context, method, pathTemplate string, args json.RawMessage) (*http.Request, error) {
	fields := map[string]json.RawMessage{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &fields); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
	}

	path, query, body := splitArgs(method, pathTemplate, fields)

	var reader io.Reader

	if len(body) > 0 {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		reader = bytes.NewReader(encoded)
	}

	target := path
	if enc := query.Encode(); enc != "" {
		target += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}

	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func splitArgs(method, pathTemplate string, fields map[string]json.RawMessage,
) (path string, query url.Values, body map[string]json.RawMessage) {
	params := make(map[string]bool)
	for _, p := range pathParams(pathTemplate) {
		params[p] = true
	}

	segs := strings.Split(pathTemplate, "/")
	query = url.Values{}
	body = map[string]json.RawMessage{}

	for key, raw := range fields {
		switch {
		case params[key]:
			// Escape so a value like "../admin" or "a/b" cannot break out of its path segment.
			substituteSegment(segs, key, url.PathEscape(scalar(raw)))
		case accessForMethod(method) == ai.ReadOnly:
			query.Set(key, scalar(raw))
		default:
			body[key] = raw
		}
	}

	return strings.Join(segs, "/"), query, body
}

func substituteSegment(segs []string, name, value string) {
	for i, seg := range segs {
		if isParamSegment(seg) && paramName(seg) == name {
			segs[i] = value
		}
	}
}

// scalar renders a JSON argument as a plain string for a path or query value: a JSON string is
// unquoted, anything else is used verbatim.
func scalar(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	return string(raw)
}

func copyAuthHeaders(dst, src http.Header) {
	for _, key := range []string{headerAuthorization, headerXAPIKey} {
		if v := src.Get(key); v != "" {
			dst.Set(key, v)
		}
	}
}

// responseCapture is an http.ResponseWriter that records the router's response for a tool call.
type responseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
	wrote  bool
}

func (c *responseCapture) Header() http.Header { return c.header }

func (c *responseCapture) WriteHeader(status int) {
	if !c.wrote {
		c.status = status
		c.wrote = true
	}
}

func (c *responseCapture) Write(b []byte) (int, error) {
	c.wrote = true

	// Cap the captured body so a large handler response cannot exhaust memory. Bytes past the cap
	// are dropped but reported written so the handler is not disrupted.
	if room := maxToolResponseBytes - c.body.Len(); room > 0 {
		if len(b) > room {
			c.body.Write(b[:room])

			return len(b), nil
		}

		return c.body.Write(b)
	}

	return len(b), nil
}
