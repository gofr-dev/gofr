package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Server implements the streamable-HTTP MCP transport over a single
// POST endpoint. It dispatches initialize/tools/list/tools/call to
// the bridge + manifest, ignoring everything else.
type Server struct {
	router  *mux.Router
	learner *Learner
	opts    BuildOptions
	info    serverInfo
	bridge  *Bridge
}

// NewServer wires the MCP dispatcher to a router (so it can see the
// registered routes when building the manifest) and a learner (so
// schemas sharpen over time). serverName/serverVersion are returned
// to clients in the initialize handshake.
func NewServer(router *mux.Router, learner *Learner, opts BuildOptions, serverName, serverVersion string) *Server {
	return &Server{
		router:  router,
		learner: learner,
		opts:    opts,
		info: serverInfo{
			Name:    serverName,
			Version: serverVersion,
		},
		bridge: NewBridge(router),
	}
}

// ServeHTTP makes Server an http.Handler. It is intended to be mounted
// at POST /mcp. We deliberately accept only POST — clients may also
// send GET for the SSE leg of the streamable transport, but tool
// invocations all flow over POST.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPC(w, rpcResponse{
			JSONRPC: jsonRPCVersion,
			Error:   &rpcError{Code: errCodeParseError, Message: "parse error"},
		})

		return
	}

	if req.JSONRPC != "" && req.JSONRPC != jsonRPCVersion {
		writeRPC(w, rpcResponse{
			JSONRPC: jsonRPCVersion,
			ID:      req.ID,
			Error:   &rpcError{Code: errCodeInvalidRequest, Message: "unsupported jsonrpc version"},
		})

		return
	}

	// Notifications (no id) get an empty 200 — JSON-RPC says we must
	// not return a response for them.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	resp := rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID}

	switch req.Method {
	case methodInitialize:
		resp.Result = s.handleInitialize(req.Params)
	case methodToolsList:
		resp.Result = s.handleToolsList()
	case methodToolsCall:
		resp.Result = s.handleToolsCall(r, req.Params)
	default:
		resp.Error = &rpcError{Code: errCodeMethodNotFound, Message: "method not found: " + req.Method}
	}

	writeRPC(w, resp)
}

func (s *Server) handleInitialize(params json.RawMessage) initializeResult {
	var p initializeParams

	_ = json.Unmarshal(params, &p) // best-effort; defaults are fine

	return initializeResult{
		ProtocolVersion: negotiate(p.ProtocolVersion),
		Capabilities: capabilities{
			Tools: &toolsCap{},
		},
		ServerInfo: s.info,
		Instructions: "GoFr MCP mode. Tools mirror the registered HTTP endpoints. " +
			"Schemas sharpen as endpoints are exercised; expect them to grow more " +
			"detailed over time.",
	}
}

func (s *Server) handleToolsList() toolsListResult {
	manifest := BuildManifest(s.router, s.learner, s.opts)

	return toolsListResult{Tools: manifest.Tools()}
}

func (s *Server) handleToolsCall(outerReq *http.Request, params json.RawMessage) any {
	var p toolsCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return rpcError{Code: errCodeInvalidParams, Message: "invalid params: " + err.Error()}
	}

	manifest := BuildManifest(s.router, s.learner, s.opts)

	method, path, ok := manifest.Find(p.Name)
	if !ok {
		return errorResult("unknown tool: " + p.Name)
	}

	return s.bridge.Call(outerReq.Context(), method, path, p.Arguments, outerReq)
}

func writeRPC(w http.ResponseWriter, resp rpcResponse) {
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(resp)
}
