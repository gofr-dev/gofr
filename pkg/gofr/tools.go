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
	"strings"

	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/mcp"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
)

var errToolStatus = errors.New("tool returned an error status")

const (
	schemaKeyType    = "type"
	schemaTypeString = "string"
	schemaTypeObject = "object"

	maxToolResponseBytes = 4 << 20 // cap a captured tool response at 4 MiB
)

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

// copyAuthHeaders forwards the caller's identity headers onto the synthesized request, using the
// canonical set the auth middleware reads so a new auth scheme is covered automatically.
func copyAuthHeaders(dst, src http.Header) {
	for _, key := range middleware.AuthHeaders() {
		if v := src.Get(key); v != "" {
			dst.Set(key, v)
		}
	}
}
