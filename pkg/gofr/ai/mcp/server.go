// Package mcp exposes an ai.Tools set as agent-callable tools over the Model Context Protocol,
// speaking JSON-RPC 2.0 over Streamable HTTP. It depends only on the standard library and
// gofr.dev/pkg/gofr/ai.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"gofr.dev/pkg/gofr/ai"
)

const maxRequestBytes = 1 << 20

var errToolPanicked = errors.New("tool execution failed")

// Server serves an ai.Tools set as MCP tools over an HTTP JSON-RPC endpoint. It implements
// http.Handler and is safe for concurrent use.
type Server struct {
	tools ai.Tools
	info  serverInfo
	hook  Hook
}

// NewServer builds an MCP server over the given tools.
func NewServer(tools ai.Tools, opts ...Option) *Server {
	o := options{name: defaultName, version: defaultVersion}

	for _, opt := range opts {
		opt(&o)
	}

	return &Server{
		tools: tools,
		info:  serverInfo{Name: o.name, Version: o.version},
		hook:  o.hook,
	}
}

// ServeHTTP handles the JSON-RPC-over-HTTP MCP endpoint.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		writeError(w, nil, codeParse, "request body too large or unreadable")

		return
	}

	var req rpcRequest
	if jerrr := json.Unmarshal(body, &req); jerrr != nil {
		writeError(w, nil, codeParse, "parse error")

		return
	}

	if req.JSONRPC != jsonRPCVersion {
		writeError(w, req.ID, codeInvalidRequest, "invalid request")

		return
	}

	if isNotification(&req) {
		w.WriteHeader(http.StatusAccepted)

		return
	}

	result, rpcErr := s.dispatch(WithHeaders(r.Context(), r.Header), &req)
	if rpcErr != nil {
		writeError(w, req.ID, rpcErr.Code, rpcErr.Message)

		return
	}

	writeResult(w, req.ID, result)
}

func (s *Server) dispatch(ctx context.Context, req *rpcRequest) (any, *rpcError) {
	switch req.Method {
	case methodInitialize:
		return s.handleInitialize(), nil
	case methodToolsList:
		return s.handleToolsList(), nil
	case methodToolsCall:
		return s.handleToolsCall(ctx, req.Params)
	case methodPing:
		return struct{}{}, nil // MCP ping: an empty result confirms liveness.
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found"}
	}
}

func (s *Server) handleInitialize() any {
	return initResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities{Tools: toolsCapability{ListChanged: false}},
		ServerInfo:      s.info,
	}
}

func (s *Server) handleToolsList() any {
	specs := s.tools.List()
	descriptors := make([]toolDescriptor, 0, len(specs))

	for _, spec := range specs {
		descriptors = append(descriptors, describe(spec))
	}

	return toolsListResult{Tools: descriptors}
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p callParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params"}
	}

	if p.Name == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "tool name is required"}
	}

	spec, ok := s.lookup(p.Name)
	if !ok {
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown tool"}
	}

	if s.hook != nil {
		if err := s.hook(ctx, spec); err != nil {
			return nil, &rpcError{Code: codeInternal, Message: err.Error()}
		}
	}

	return s.runTool(ctx, p), nil
}

func (s *Server) runTool(ctx context.Context, p callParams) *toolResult {
	args := p.Arguments
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}

	res, err := s.callTool(ctx, p.Name, args)
	if err != nil {
		return errorResult(err.Error())
	}

	raw, err := res.JSON()
	if err != nil {
		return errorResult("failed to encode tool result")
	}

	return &toolResult{
		Content: []textContent{{Type: contentTypeText, Text: string(raw)}},
		IsError: false,
	}
}

func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) (res ai.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errToolPanicked
		}
	}()

	return s.tools.Call(ctx, name, args)
}

func (s *Server) lookup(name string) (ai.ToolSpec, bool) {
	for _, spec := range s.tools.List() {
		if spec.Name == name {
			return spec, true
		}
	}

	return ai.ToolSpec{}, false
}

func describe(spec ai.ToolSpec) toolDescriptor {
	schema := spec.InputSchema
	if len(schema) == 0 {
		schema = json.RawMessage(defaultSchema)
	}

	d := toolDescriptor{Name: spec.Name, Description: spec.Description, InputSchema: schema}
	if spec.Access == ai.ReadOnly {
		d.Annotations = &annotations{ReadOnlyHint: true}
	}

	return d
}

func errorResult(text string) *toolResult {
	return &toolResult{
		Content: []textContent{{Type: contentTypeText, Text: text}},
		IsError: true,
	}
}
