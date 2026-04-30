package mcp

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/gorilla/mux"
)

// pathParamPattern matches a `{name}` segment, optionally with a
// regex constraint like `{id:[0-9]+}`. We strip the constraint when
// generating tool names because clients can't use regex anyway.
var pathParamPattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)(?::[^}]+)?\}`)

// builtInPathPrefixes are routes that exist on every GoFr app for
// framework purposes. They're never useful as MCP tools.
var builtInPathPrefixes = []string{
	"/.well-known/",
	"/swagger",
	"/favicon.ico",
	"/static/",
	"/mcp",
}

// Manifest is the assembled view used by the server: the routes that
// should be exposed plus their sharpened schemas. Built fresh on each
// tools/list call so newly-learned schemas appear without restart.
type Manifest struct {
	tools []Tool
}

// BuildOptions controls which routes appear in the manifest.
type BuildOptions struct {
	// AllowMutations exposes POST/PUT/PATCH/DELETE in addition to GET.
	// False by default — mutations require explicit opt-in via
	// MCP_ENABLED=full so an LLM can't accidentally delete data.
	AllowMutations bool
}

// BuildManifest walks the mux router, applies the policy in opts, and
// merges learned schemas from learner. learner may be nil; in that
// case schemas degrade to "object" for everything but path params.
func BuildManifest(router *mux.Router, learner *Learner, opts BuildOptions) *Manifest {
	if router == nil {
		return &Manifest{}
	}

	allowedMethods := map[string]bool{http.MethodGet: true, http.MethodHead: true}
	if opts.AllowMutations {
		for _, m := range []string{
			http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		} {
			allowedMethods[m] = true
		}
	}

	var tools []Tool

	_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}

		if isBuiltInPath(path) {
			return nil
		}

		methods, err := route.GetMethods()
		if err != nil || len(methods) == 0 {
			return nil
		}

		for _, m := range methods {
			if !allowedMethods[m] {
				continue
			}

			tools = append(tools, makeTool(m, path, learner))
		}

		return nil
	})

	// Stable ordering — clients display tools in the order we send
	// them, and a stable order makes diffs reviewable.
	slices.SortFunc(tools, func(a, b Tool) int {
		return strings.Compare(a.Name, b.Name)
	})

	return &Manifest{tools: tools}
}

// Tools returns a copy of the tool list ready to send to the client.
func (m *Manifest) Tools() []Tool {
	out := make([]Tool, len(m.tools))
	copy(out, m.tools)

	return out
}

// Find returns the route key (METHOD path) for a given tool name, or
// "" if the name is unknown. Used by the call dispatcher.
func (m *Manifest) Find(toolName string) (method, path string, ok bool) {
	for _, t := range m.tools {
		if t.Name == toolName {
			// Stash method and path in description-prefixed form was
			// considered; instead we re-parse the description, which
			// is "METHOD path".
			parts := strings.SplitN(t.Description, " ", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], true
			}
		}
	}

	return "", "", false
}

func makeTool(method, path string, learner *Learner) Tool {
	pathParams := extractPathParams(path)

	t := Tool{
		Name:        toolName(method, path),
		Description: method + " " + path,
		InputSchema: Schema{"type": "object", "properties": map[string]any{}},
	}

	props, _ := t.InputSchema["properties"].(map[string]any)

	var required []string

	// Path params are always required and always strings (mux matches
	// them as strings).
	for _, p := range pathParams {
		props[p] = Schema{"type": "string"}

		required = append(required, p)
	}

	// If the learner has a recorded body type, fold it in. Otherwise
	// allow an opaque body for POST/PUT/PATCH so the LLM can still
	// invoke the tool.
	if learner != nil {
		schemas := learner.Schemas()
		if rs, ok := schemas[routeKey(method, path)]; ok {
			for _, q := range rs.QueryKeys {
				if _, exists := props[q]; !exists {
					props[q] = Schema{"type": "string"}
				}
			}

			if len(rs.Body) > 0 && needsBody(method) {
				props["body"] = rs.Body
				required = append(required, "body")
			}

			if len(rs.Output) > 0 {
				t.OutputSchema = rs.Output
			}
		}
	}

	if needsBody(method) {
		// Even without a learned schema, advertise "body" as accepted.
		// Loose schemas let the LLM still send something.
		if _, ok := props["body"]; !ok {
			props["body"] = Schema{"type": "object"}
		}
	}

	if len(required) > 0 {
		t.InputSchema["required"] = required
	}

	return t
}

// toolName builds an MCP-safe name from method and path. MCP requires
// names to match ^[a-zA-Z][a-zA-Z0-9_-]*$, so slashes and braces are
// stripped. Verb prefix matches the HTTP method (lowercased).
func toolName(method, path string) string {
	verb := strings.ToLower(method)

	// Strip path-param regex constraints, then drop braces.
	cleaned := pathParamPattern.ReplaceAllString(path, "$1")
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	cleaned = strings.ReplaceAll(cleaned, ".", "_")

	if cleaned == "" {
		return verb + "_root"
	}

	return verb + "_" + cleaned
}

func extractPathParams(path string) []string {
	matches := pathParamPattern.FindAllStringSubmatch(path, -1)

	params := make([]string, 0, len(matches))
	for _, m := range matches {
		params = append(params, m[1])
	}

	return params
}

func needsBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func isBuiltInPath(path string) bool {
	for _, prefix := range builtInPathPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}
