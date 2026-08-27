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
	"path"
	"strings"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/mcp"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
)

var (
	errToolStatus      = errors.New("tool returned an error status")
	errUnsafePathParam = errors.New("unsafe path parameter")
)

const (
	schemaKeyType    = "type"
	schemaTypeString = "string"
	schemaTypeObject = "object"

	// bodyKey is the tool argument that carries a QUERY request body (the query payload).
	bodyKey = "body"

	maxToolResponseBytes = 4 << 20 // cap a captured tool response at 4 MiB
)

// MethodQuery is the HTTP QUERY method (RFC 10008), re-exported from
// gofr.dev/pkg/gofr/http so callers can reference gofr.MethodQuery the way they
// use http.MethodGet from the stdlib, instead of hardcoding the "QUERY" string.
const MethodQuery = gofrHTTP.MethodQuery

// registerTools builds the router-backed tool provider and installs it so both the MCP server and
// ctx.LLM().Tools() expose the app's handlers. It is the tool-exposure step, independent of the MCP
// transport that EnableMCP layers on top.
func (a *App) registerTools(cfg *mcpConfig) ai.Tools {
	tools := &routerTools{app: a, cfg: cfg}
	a.container.SetTools(tools)

	return tools
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

	rec := gofrHTTP.NewResponseCapture(maxToolResponseBytes)
	rt.app.httpServer.router.ServeHTTP(rec, req)

	if rec.Status() >= http.StatusBadRequest {
		return ai.Result{}, fmt.Errorf("%w: tool %q returned status %d", errToolStatus, name, rec.Status())
	}

	return ai.NewResult(json.RawMessage(rec.Body())), nil
}

func (rt *routerTools) specFor(method, pathTemplate string) (ai.ToolSpec, bool) {
	if rt.cfg.exclude[pathTemplate] || isFrameworkRoute(pathTemplate) {
		return ai.ToolSpec{}, false
	}

	// Only safe handlers are exposed as tools: read-only methods (GET/HEAD/OPTIONS) and QUERY
	// (RFC 10008), which is safe and idempotent. Write handlers (POST/PUT/PATCH/DELETE) are never
	// exposed, so an agent cannot mutate state through the MCP surface.
	if !isExposableMethod(method) {
		return ai.ToolSpec{}, false
	}

	return ai.ToolSpec{
		Name:        toolName(method, pathTemplate),
		Description: method + " " + pathTemplate,
		InputSchema: toolSchema(method, pathTemplate),
		Access:      ai.ReadOnly,
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

func isReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// isExposableMethod reports whether a route's method may be exposed as an agent tool. Read-only
// methods (GET/HEAD/OPTIONS) and QUERY (RFC 10008 — safe and idempotent, carrying its input in the
// request body) qualify; write methods never do.
func isExposableMethod(method string) bool {
	return isReadOnlyMethod(method) || method == MethodQuery
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

// toolSchema builds the JSON Schema for a tool's arguments from the route's path parameters, plus a
// required "body" object for QUERY tools (RFC 10008 carries the query in the request body). A route
// with no path parameters and no body gets no schema (nil).
func toolSchema(method, pathTemplate string) json.RawMessage {
	params := pathParams(pathTemplate)
	needsBody := method == MethodQuery

	if len(params) == 0 && !needsBody {
		return nil
	}

	props := map[string]any{}
	required := make([]string, 0, len(params)+1)

	for _, p := range params {
		props[p] = map[string]string{schemaKeyType: schemaTypeString}
		required = append(required, p) // path params are always required
	}

	if needsBody {
		props[bodyKey] = map[string]any{
			schemaKeyType: schemaTypeObject,
			"description": "The QUERY request body (RFC 10008): the query payload sent to the endpoint.",
		}
		required = append(required, bodyKey)
	}

	schema := map[string]any{
		schemaKeyType: schemaTypeObject,
		"properties":  props,
		"required":    required,
	}

	out, _ := json.Marshal(schema)

	return out
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

	// QUERY tools carry the query payload in the request body; extract it before the remaining
	// arguments are mapped to path/query so it is not double-counted as a query value.
	var (
		body    io.Reader = http.NoBody
		hasBody bool
	)

	if method == MethodQuery {
		if raw, ok := fields[bodyKey]; ok {
			delete(fields, bodyKey)

			body = bytes.NewReader(raw)
			hasBody = true
		}
	}

	reqPath, query, err := splitArgs(pathTemplate, fields)
	if err != nil {
		return nil, err
	}

	target := reqPath
	if enc := query.Encode(); enc != "" {
		target += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}

	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// splitArgs maps a tool's arguments to a path and query string. Path parameters are substituted into
// their route segment; every other argument becomes a query value. For QUERY tools the body argument
// is extracted by the caller before this runs, so only path and query values remain here.
func splitArgs(pathTemplate string, fields map[string]json.RawMessage,
) (reqPath string, query url.Values, err error) {
	params := make(map[string]bool)
	for _, p := range pathParams(pathTemplate) {
		params[p] = true
	}

	segs := strings.Split(pathTemplate, "/")
	query = url.Values{}

	for key, raw := range fields {
		if !params[key] {
			addQueryValue(query, key, raw)
			continue
		}

		// A path parameter must stay within its own segment. Escaping alone is not enough:
		// url.PathEscape leaves "." intact, so "../secret" would survive as "..%2Fsecret" and the
		// router's path.Clean could still collapse it out of the intended route. Reject any value
		// that is not a single safe segment before it is placed in the path.
		val := scalar(raw)
		if !safePathSegment(val) {
			return "", nil, fmt.Errorf("%w: %q=%q", errUnsafePathParam, key, val)
		}

		substituteSegment(segs, key, url.PathEscape(val))
	}

	return strings.Join(segs, "/"), query, nil
}

// isFrameworkRoute reports whether a route is registered by the framework itself (health/liveness
// probes, the favicon) rather than the application, so it is never exposed as an agent tool.
func isFrameworkRoute(pathTemplate string) bool {
	return pathTemplate == "/favicon.ico" || strings.HasPrefix(pathTemplate, "/.well-known")
}

// safePathSegment reports whether a value is a single path segment safe to substitute into a route:
// non-empty, containing no separator, and unchanged by path.Clean (which rejects "." and ".."
// traversal tokens, since path.Clean("/.") and path.Clean("/..") both collapse to "/").
func safePathSegment(v string) bool {
	if v == "" || strings.ContainsAny(v, "/\\") {
		return false
	}

	return path.Clean("/"+v) == "/"+v
}

// addQueryValue maps a JSON argument to query values: an array becomes repeated key=value pairs so
// ctx.Params returns each element; a scalar becomes a single value.
func addQueryValue(query url.Values, key string, raw json.RawMessage) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, elem := range arr {
			query.Add(key, scalar(elem))
		}

		return
	}

	query.Set(key, scalar(raw))
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

// copyAuthHeaders forwards the caller's identity headers onto the synthesized request, using the
// canonical set the auth middleware reads so a new auth scheme is covered automatically.
func copyAuthHeaders(dst, src http.Header) {
	for _, key := range middleware.AuthHeaders() {
		if v := src.Get(key); v != "" {
			dst.Set(key, v)
		}
	}
}
