package mcp

import "encoding/json"

// Minimal JSON-RPC 2.0 + MCP message types. We implement only what the
// streamable-HTTP transport needs for tool exposure: initialize,
// tools/list, tools/call. Notifications are accepted and dropped on
// the floor. The full MCP spec covers resources, prompts, sampling,
// and roots — none of which apply to "expose existing endpoints," so
// we deliberately omit them rather than carry unused surface.
//
// Reference: https://spec.modelcontextprotocol.io
const (
	// JSON-RPC 2.0 always uses this version string.
	jsonRPCVersion = "2.0"

	// MCP protocol versions we know how to speak. Negotiation falls
	// back to whichever the client requested if it's in this set; if
	// not, we return our newest. Both values shipped in late-2025
	// clients (Claude Desktop, Cursor, Continue) so this list is what
	// the field has settled on.
	protocolVersionLatest = "2025-06-18"
	protocolVersionFallback = "2024-11-05"

	methodInitialize = "initialize"
	methodToolsList  = "tools/list"
	methodToolsCall  = "tools/call"

	// Standard JSON-RPC 2.0 error codes.
	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternal       = -32603
)

// rpcRequest is the inbound JSON-RPC envelope.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is the outbound envelope. Either Result or Error is set
// — JSON-RPC requires that exactly one is present.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// initializeResult is what we return when the client opens the session.
// Capabilities advertise that we support tools but not resources or
// prompts — which is true.
type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    capabilities   `json:"capabilities"`
	ServerInfo      serverInfo     `json:"serverInfo"`
	Instructions    string         `json:"instructions,omitempty"`
}

type capabilities struct {
	Tools *toolsCap `json:"tools,omitempty"`
}

type toolsCap struct {
	// listChanged would be true if we sent notifications when the tool
	// list changes. We don't (yet), so omit it.
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// initializeParams is what the client sends. We accept whatever
// version they requested if we know it.
type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// Tool is the MCP-shaped description of an exposed endpoint. The
// fields here mirror the spec field-for-field.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema Schema `json:"inputSchema"`
	// OutputSchema is supported by newer clients; keep it omitempty
	// so older clients don't choke on an empty value.
	OutputSchema Schema `json:"outputSchema,omitempty"`
}

type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult is what we return for tools/call. content is always
// at least one entry — clients are picky about empty arrays.
type CallToolResult struct {
	Content           []Content `json:"content"`
	IsError           bool      `json:"isError,omitempty"`
	StructuredContent any       `json:"structuredContent,omitempty"`
}

// Content is a tagged union per the MCP spec. We only ever produce
// "text" today; image/resource forms are reserved for future use.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// negotiate picks the protocol version we'll respond with given what
// the client asked for. Always succeed — if their version is unknown,
// answer with our latest and let them decide whether to continue.
func negotiate(clientVersion string) string {
	switch clientVersion {
	case protocolVersionLatest, protocolVersionFallback:
		return clientVersion
	default:
		return protocolVersionLatest
	}
}
