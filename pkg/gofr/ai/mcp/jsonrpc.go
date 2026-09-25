package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
)

const (
	jsonRPCVersion  = "2.0"
	protocolVersion = "2025-06-18"

	methodInitialize = "initialize"
	methodToolsList  = "tools/list"
	methodToolsCall  = "tools/call"
	methodPing       = "ping"

	contentTypeText = "text"
	defaultSchema   = `{"type":"object"}`
)

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603

	msgInternal = "internal error"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func isNotification(req *rpcRequest) bool {
	return len(req.ID) == 0
}

func idOrNull(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}

	return id
}

func (s *Server) writeResult(w http.ResponseWriter, id json.RawMessage, result any) {
	s.writeJSON(w, rpcResponse{JSONRPC: jsonRPCVersion, ID: idOrNull(id), Result: result})
}

func (s *Server) writeError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	s.writeJSON(w, rpcResponse{JSONRPC: jsonRPCVersion, ID: idOrNull(id), Error: &rpcError{Code: code, Message: msg}})
}

// writeJSON encodes resp into a buffer before touching w, so a value that cannot be encoded is
// reported to the client as a JSON-RPC internal error instead of an empty 200, and nothing
// partial is ever written.
func (s *Server) writeJSON(w http.ResponseWriter, resp rpcResponse) {
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		s.logf("failed to encode MCP response: %v", err)

		buf.Reset()

		fallback := rpcResponse{JSONRPC: jsonRPCVersion, ID: resp.ID, Error: &rpcError{Code: codeInternal, Message: msgInternal}}
		if err = json.NewEncoder(&buf).Encode(fallback); err != nil {
			// Only reachable with an ID that is not valid JSON, which ServeHTTP never produces.
			s.logf("failed to encode MCP internal-error response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if _, err := w.Write(buf.Bytes()); err != nil {
		s.logf("failed to write MCP response: %v", err)
	}
}
