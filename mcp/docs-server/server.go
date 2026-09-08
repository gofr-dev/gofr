package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

const (
	// A single JSON-RPC message can carry a whole documentation page, so
	// the default 64 KB scanner limit is far too small.
	initialBufferBytes = 64 * 1024
	maxMessageBytes    = 8 * 1024 * 1024
)

// Tool names, as the model sees them.
const (
	toolSearchDocs   = "search_docs"
	toolGetDoc       = "get_doc"
	toolListSections = "list_sections"

	jsonRPCVersion   = "2.0"
	schemaTypeObject = "object"
)

// JSON-RPC 2.0 error codes used by MCP.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type server struct {
	corpus *corpus
}

func newServer(client *http.Client, url string) *server {
	return &server{corpus: newCorpus(client, url)}
}

// serve reads newline-delimited JSON-RPC messages until EOF.
//
// A notification (no id) gets no reply, per JSON-RPC — `notifications/
// initialized` is one, and answering it makes strict clients disconnect.
func (s *server) serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, initialBufferBytes), maxMessageBytes)

	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		resp, ok := s.handleLine(ctx, []byte(line))
		if !ok {
			continue
		}

		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
	}

	return scanner.Err()
}

// handleLine returns the response to send, or ok=false when the message
// is a notification that must not be answered.
func (s *server) handleLine(ctx context.Context, line []byte) (response, bool) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse(nil, codeParseError, "invalid JSON"), true
	}

	if req.Method == "" {
		return errorResponse(req.ID, codeInvalidRequest, "missing method"), true
	}

	if len(req.ID) == 0 {
		return response{}, false
	}

	result, rpcErr := s.dispatch(ctx, &req)
	if rpcErr != nil {
		return response{JSONRPC: jsonRPCVersion, ID: req.ID, Error: rpcErr}, true
	}

	return response{JSONRPC: jsonRPCVersion, ID: req.ID, Result: result}, true
}

func (s *server) dispatch(ctx context.Context, req *request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVer,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": serverName, "version": serverVersion},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefinitions()}, nil
	case "tools/call":
		return s.callTool(ctx, req.Params)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown method: " + req.Method}
	}
}

type toolCall struct {
	Name      string `json:"name"`
	Arguments struct {
		Query string `json:"query"`
		Path  string `json:"path"`
		Limit int    `json:"limit"`
	} `json:"arguments"`
}

func (s *server) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var call toolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{Code: codeInvalidRequest, Message: "invalid tool arguments"}
	}

	pages, err := s.corpus.load(ctx)
	if err != nil {
		return nil, &rpcError{Code: codeInternalError, Message: err.Error()}
	}

	switch call.Name {
	case toolSearchDocs:
		return searchDocs(pages, &call)
	case toolGetDoc:
		return getDoc(pages, &call)
	case toolListSections:
		return textResult(renderSections(sections(pages))), nil
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown tool: " + call.Name}
	}
}

func searchDocs(pages []page, call *toolCall) (any, *rpcError) {
	if strings.TrimSpace(call.Arguments.Query) == "" {
		return toolError("search_docs requires a non-empty `query`."), nil
	}

	limit := call.Arguments.Limit
	if limit <= 0 {
		limit = defaultResults
	}

	results := search(pages, call.Arguments.Query, limit)
	if len(results) == 0 {
		return toolError(fmt.Sprintf(
			"No GoFr documentation matched %q. Try fewer or broader terms, or call list_sections.",
			call.Arguments.Query)), nil
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%d result(s) for %q:\n\n", len(results), call.Arguments.Query)

	for _, r := range results {
		fmt.Fprintf(&b, "## %s\n%s\n\n%s\n\n---\n\n",
			r.Page.Title, r.Page.URL(), excerpt(r.Page.Content, call.Arguments.Query))
	}

	return textResult(b.String()), nil
}

func getDoc(pages []page, call *toolCall) (any, *rpcError) {
	if strings.TrimSpace(call.Arguments.Path) == "" {
		return toolError("get_doc requires a `path`, e.g. /docs/quick-start/introduction."), nil
	}

	p, ok := findPage(pages, call.Arguments.Path)
	if !ok {
		return toolError(fmt.Sprintf(
			"No GoFr documentation page at %q. Call search_docs or list_sections to find the right path.",
			call.Arguments.Path)), nil
	}

	return textResult(fmt.Sprintf("%s\n\n%s\n", p.URL(), p.Content)), nil
}

func renderSections(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}

	sort.Strings(names)

	var b strings.Builder

	b.WriteString("GoFr documentation sections:\n\n")

	for _, name := range names {
		fmt.Fprintf(&b, "- /%s (%d pages)\n", name, counts[name])
	}

	return b.String()
}

// The MCP tool-result wire shape.
type toolResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string) toolResult {
	return toolResult{Content: []contentBlock{{Type: "text", Text: text}}}
}

// toolError reports a problem with the call itself. MCP models these as a
// successful result carrying isError, not as a JSON-RPC error, so the
// model can read the message and correct its next call.
func toolError(text string) toolResult {
	result := textResult(text)
	result.IsError = true

	return result
}

func errorResponse(id json.RawMessage, code int, message string) response {
	return response{JSONRPC: jsonRPCVersion, ID: id, Error: &rpcError{Code: code, Message: message}}
}

// The MCP tool-definition wire shape. Modeled as types rather than
// map[string]any so the compiler checks the payload the client parses.
type tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema toolSchema `json:"inputSchema"`
}

type toolSchema struct {
	Type       string              `json:"type"`
	Properties map[string]property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
}

func toolDefinitions() []tool {
	return []tool{
		{
			Name: toolSearchDocs,
			Description: "Full-text search across all GoFr documentation. Use this first when answering " +
				"a question about the GoFr Go framework — routing, handlers, datasources, observability, " +
				"gRPC, GraphQL, WebSockets, Pub/Sub, migrations, deployment, or migrating from another framework.",
			InputSchema: toolSchema{
				Type: schemaTypeObject,
				Properties: map[string]property{
					"query": {
						Type:        "string",
						Description: "Search terms, e.g. 'kafka consumer' or 'custom metrics'.",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results.",
						Default:     defaultResults,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name: toolGetDoc,
			Description: "Fetch one GoFr documentation page in full, as Markdown. Use after search_docs " +
				"when an excerpt is not enough.",
			InputSchema: toolSchema{
				Type: schemaTypeObject,
				Properties: map[string]property{
					"path": {
						Type:        "string",
						Description: "Site path, e.g. '/docs/quick-start/introduction'.",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name: toolListSections,
			Description: "List the GoFr documentation sections and how many pages each contains. " +
				"Use to orient before searching.",
			InputSchema: toolSchema{
				Type:       schemaTypeObject,
				Properties: map[string]property{},
			},
		},
	}
}
