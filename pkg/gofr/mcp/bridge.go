package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// Bridge translates an MCP tools/call into a synthetic HTTP request,
// runs it through the existing router (so all middleware applies),
// and packages the response as a CallToolResult.
//
// The point: the user's handler does not know the call came from MCP.
// Auth, RBAC, OTel spans, rate limit, datasources — everything works
// the same as for a real HTTP call.
type Bridge struct {
	router http.Handler
}

// NewBridge wraps an http.Handler (the router) for use as the
// synthetic-call target.
func NewBridge(router http.Handler) *Bridge {
	return &Bridge{router: router}
}

// Call invokes the named tool. method/path come from the manifest;
// rawArgs is the raw JSON `arguments` object the MCP client sent.
// outerReq, when non-nil, is the inbound /mcp HTTP request — we lift
// auth headers from it onto the synthetic request so existing auth
// middleware sees them.
func (b *Bridge) Call(ctx context.Context, method, path string, rawArgs json.RawMessage, outerReq *http.Request) CallToolResult {
	args := map[string]any{}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return errorResult(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	resolvedPath, query, err := substitutePath(path, args)
	if err != nil {
		return errorResult(err.Error())
	}

	body, err := bodyBytes(method, args)
	if err != nil {
		return errorResult(err.Error())
	}

	target := resolvedPath
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return errorResult(fmt.Sprintf("build request: %v", err))
	}

	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	// Forward auth + a few other useful headers from the outer MCP
	// request so middleware chains (JWT, API key) authenticate the
	// synthetic call with the same credentials that opened the MCP
	// session.
	if outerReq != nil {
		for _, h := range []string{
			"Authorization",
			"X-Api-Key",
			"X-Forwarded-For",
			"X-Request-Id",
			"Cookie",
		} {
			if v := outerReq.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}
	}

	rec := httptest.NewRecorder()
	b.router.ServeHTTP(rec, req)

	return packageResponse(rec)
}

// substitutePath replaces {name} placeholders in the path template
// with values from args. Used keys are removed from args (in place);
// remaining args become the query string.
func substitutePath(path string, args map[string]any) (string, url.Values, error) {
	matches := pathParamPattern.FindAllStringSubmatchIndex(path, -1)

	if len(matches) == 0 {
		return path, argsToQuery(args), nil
	}

	// Build the resolved path by walking matches in order, slicing
	// the original path between them.
	var b strings.Builder

	last := 0

	for _, m := range matches {
		// m = [fullStart, fullEnd, nameStart, nameEnd, regexStart?, regexEnd?]
		b.WriteString(path[last:m[0]])

		name := path[m[2]:m[3]]

		v, ok := args[name]
		if !ok {
			return "", nil, fmt.Errorf("missing required path parameter %q", name)
		}

		b.WriteString(url.PathEscape(stringify(v)))

		delete(args, name)

		last = m[1]
	}

	b.WriteString(path[last:])

	return b.String(), argsToQuery(args), nil
}

// argsToQuery flattens remaining args into URL query. The "body" key
// is reserved for the request body and not included as a query param.
func argsToQuery(args map[string]any) url.Values {
	q := url.Values{}

	for k, v := range args {
		if k == "body" {
			continue
		}

		q.Set(k, stringify(v))
	}

	return q
}

func bodyBytes(method string, args map[string]any) ([]byte, error) {
	if !needsBody(method) {
		return nil, nil
	}

	body, ok := args["body"]
	if !ok {
		// Empty JSON body is acceptable — handlers that don't bind
		// will ignore it; ones that do will get the standard "EOF"
		// error and return their own validation message.
		return []byte("{}"), nil
	}

	switch v := body.(type) {
	case string:
		// Some clients pre-stringify the body. Trust it as-is.
		return []byte(v), nil
	default:
		return json.Marshal(v)
	}
}

func packageResponse(rec *httptest.ResponseRecorder) CallToolResult {
	resp := rec.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	result := CallToolResult{
		Content: []Content{{Type: "text", Text: string(body)}},
		IsError: resp.StatusCode >= 400,
	}

	// If the body parses as JSON, also expose it via structuredContent
	// so newer clients can use the typed form. Older clients ignore
	// the field.
	if len(body) > 0 && isJSONContent(resp.Header.Get("Content-Type")) {
		var structured any
		if err := json.Unmarshal(body, &structured); err == nil {
			result.StructuredContent = structured
		}
	}

	return result
}

func errorResult(msg string) CallToolResult {
	return CallToolResult{
		Content: []Content{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		// Fall back to JSON encoding so numbers, bools, etc. round-trip.
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}

		s := string(b)

		// Strip surrounding quotes for plain string-encoded values so
		// the resulting URL/query is human-readable.
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}

		return s
	}
}

func isJSONContent(ct string) bool {
	ct = strings.ToLower(strings.SplitN(ct, ";", 2)[0])

	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}
