package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
)

var (
	errDBUnavailable = errors.New("db unavailable")
	errAccessDenied  = errors.New("access denied")
)

type fakeTools struct {
	specs   []ai.ToolSpec
	call    func(ctx context.Context, name string, args json.RawMessage) (ai.Result, error)
	callLog []string
	mu      sync.Mutex
}

func (f *fakeTools) List() []ai.ToolSpec { return f.specs }

func (f *fakeTools) Only(_ ...string) ai.Tools { return f }

func (f *fakeTools) Call(ctx context.Context, name string, args json.RawMessage) (ai.Result, error) {
	f.mu.Lock()
	f.callLog = append(f.callLog, name)
	f.mu.Unlock()

	if f.call != nil {
		return f.call(ctx, name, args)
	}

	return ai.NewResult(map[string]string{"ok": name}), nil
}

func readTool() ai.ToolSpec {
	return ai.ToolSpec{Name: "get_user", Description: "reads a user", Access: ai.ReadOnly}
}

func writeTool() ai.ToolSpec {
	return ai.ToolSpec{
		Name:        "create_user",
		Description: "creates a user",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
		Access:      ai.Write,
	}
}

func post(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) rpcResponse {
	t.Helper()

	var resp rpcResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	return resp
}

func TestServer_Initialize(t *testing.T) {
	s := NewServer(&fakeTools{}, WithServerInfo("svc", "9.9.9"))

	rec := post(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)

	require.Equal(t, http.StatusOK, rec.Code)

	resp := decode(t, rec)
	require.Nil(t, resp.Error)
	require.Equal(t, "1", string(resp.ID))

	var got initResult

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, protocolVersion, got.ProtocolVersion)
	assert.Equal(t, "svc", got.ServerInfo.Name)
	assert.Equal(t, "9.9.9", got.ServerInfo.Version)
	assert.False(t, got.Capabilities.Tools.ListChanged)
}

func TestServer_Ping(t *testing.T) {
	s := NewServer(&fakeTools{})

	rec := post(t, s, `{"jsonrpc":"2.0","id":7,"method":"ping"}`)

	require.Equal(t, http.StatusOK, rec.Code)

	resp := decode(t, rec)
	require.Nil(t, resp.Error)
	assert.Equal(t, "7", string(resp.ID))
	assert.NotNil(t, resp.Result)
}

func TestServer_ToolsList(t *testing.T) {
	s := NewServer(&fakeTools{specs: []ai.ToolSpec{readTool(), writeTool()}})

	rec := post(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var got toolsListResult

	raw, err := json.Marshal(decode(t, rec).Result)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &got))

	require.Len(t, got.Tools, 2)

	assert.Equal(t, "get_user", got.Tools[0].Name)
	assert.JSONEq(t, defaultSchema, string(got.Tools[0].InputSchema))
	require.NotNil(t, got.Tools[0].Annotations)
	assert.True(t, got.Tools[0].Annotations.ReadOnlyHint)

	assert.Equal(t, "create_user", got.Tools[1].Name)
	assert.Nil(t, got.Tools[1].Annotations)
	assert.Contains(t, string(got.Tools[1].InputSchema), "properties")
}

func TestServer_ToolsCall(t *testing.T) {
	tests := []struct {
		name        string
		tools       *fakeTools
		body        string
		wantRPCCode int
		wantResult  bool
		wantIsError bool
		wantText    string
	}{
		{
			name:       "success",
			tools:      &fakeTools{specs: []ai.ToolSpec{readTool()}},
			body:       `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_user","arguments":{"id":"1"}}}`,
			wantResult: true,
			wantText:   `{"ok":"get_user"}`,
		},
		{
			name:        "unknown-tool",
			tools:       &fakeTools{specs: []ai.ToolSpec{readTool()}},
			body:        `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
			wantRPCCode: codeMethodNotFound,
		},
		{
			name: "tool returns error is reported in band",
			tools: &fakeTools{
				specs: []ai.ToolSpec{readTool()},
				call: func(context.Context, string, json.RawMessage) (ai.Result, error) {
					return ai.Result{}, errDBUnavailable
				},
			},
			body:        `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`,
			wantResult:  true,
			wantIsError: true,
			wantText:    "db unavailable",
		},
		{
			name:        "invalid-params",
			tools:       &fakeTools{specs: []ai.ToolSpec{readTool()}},
			body:        `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":"notanobject"}`,
			wantRPCCode: codeInvalidParams,
		},
		{
			name:        "missing tool name",
			tools:       &fakeTools{specs: []ai.ToolSpec{readTool()}},
			body:        `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"arguments":{}}}`,
			wantRPCCode: codeInvalidParams,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(tc.tools)
			resp := decode(t, post(t, s, tc.body))

			if tc.wantRPCCode != 0 {
				require.NotNil(t, resp.Error)
				assert.Equal(t, tc.wantRPCCode, resp.Error.Code)

				return
			}

			require.Nil(t, resp.Error)

			var res toolResult

			raw, err := json.Marshal(resp.Result)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &res))

			assert.Equal(t, tc.wantIsError, res.IsError)
			require.Len(t, res.Content, 1)
			assert.Contains(t, res.Content[0].Text, tc.wantText)
		})
	}
}

func TestServer_HookAborts(t *testing.T) {
	called := false

	tools := &fakeTools{
		specs: []ai.ToolSpec{readTool()},
		call: func(context.Context, string, json.RawMessage) (ai.Result, error) {
			called = true

			return ai.NewResult(nil), nil
		},
	}

	s := NewServer(tools, WithHook(func(context.Context, ai.ToolSpec) error { return errAccessDenied }))

	resp := decode(t, post(t, s, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`))

	require.NotNil(t, resp.Error)
	assert.Equal(t, codeInternal, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "access denied")
	assert.False(t, called, "tool must not be called once the hook aborts")
}

func TestServer_Notification(t *testing.T) {
	s := NewServer(&fakeTools{})

	rec := post(t, s, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestServer_NonPost(t *testing.T) {
	s := NewServer(&fakeTools{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/mcp", http.NoBody)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.MethodPost, rec.Header().Get("Allow"))
}

func TestServer_MalformedJSON(t *testing.T) {
	s := NewServer(&fakeTools{})

	resp := decode(t, post(t, s, `{not json`))

	require.NotNil(t, resp.Error)
	assert.Equal(t, codeParse, resp.Error.Code)
	assert.Equal(t, "null", string(resp.ID))
}

func TestServer_UnknownMethod(t *testing.T) {
	s := NewServer(&fakeTools{})

	resp := decode(t, post(t, s, `{"jsonrpc":"2.0","id":9,"method":"does/not/exist"}`))

	require.NotNil(t, resp.Error)
	assert.Equal(t, codeMethodNotFound, resp.Error.Code)
}

func TestServer_InvalidJSONRPCVersion(t *testing.T) {
	s := NewServer(&fakeTools{})

	resp := decode(t, post(t, s, `{"jsonrpc":"1.0","id":10,"method":"initialize"}`))

	require.NotNil(t, resp.Error)
	assert.Equal(t, codeInvalidRequest, resp.Error.Code)
}

func TestServer_HeaderPropagation(t *testing.T) {
	var seen string

	tools := &fakeTools{
		specs: []ai.ToolSpec{readTool()},
		call: func(ctx context.Context, _ string, _ json.RawMessage) (ai.Result, error) {
			seen = HeadersFromContext(ctx).Get("Authorization")

			return ai.NewResult(nil), nil
		},
	}

	s := NewServer(tools)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`))
	req.Header.Set("Authorization", "Bearer secret-token")

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Bearer secret-token", seen)
}

func TestServer_OversizedBody(t *testing.T) {
	s := NewServer(&fakeTools{})

	huge := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":"` + strings.Repeat("a", maxRequestBytes+1) + `"}`
	resp := decode(t, post(t, s, huge))

	require.NotNil(t, resp.Error)
	assert.Equal(t, codeParse, resp.Error.Code)
}

func TestServer_PanicContained(t *testing.T) {
	tools := &fakeTools{
		specs: []ai.ToolSpec{readTool()},
		call: func(context.Context, string, json.RawMessage) (ai.Result, error) {
			panic("boom")
		},
	}

	s := NewServer(tools)

	resp := decode(t, post(t, s, `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`))

	require.Nil(t, resp.Error)

	var res toolResult

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &res))

	assert.True(t, res.IsError)
	require.Len(t, res.Content, 1)
	assert.Equal(t, errToolPanicked.Error(), res.Content[0].Text)
	assert.NotContains(t, res.Content[0].Text, "boom")
}

func TestServer_HostileID(t *testing.T) {
	s := NewServer(&fakeTools{specs: []ai.ToolSpec{readTool()}})

	hostileID := `"` + strings.Repeat("9", 5000) + `"`
	resp := decode(t, post(t, s, `{"jsonrpc":"2.0","id":`+hostileID+`,"method":"tools/list"}`))

	require.Nil(t, resp.Error)
	assert.Equal(t, hostileID, string(resp.ID))
}

func TestServer_ConcurrentCalls(t *testing.T) {
	s := NewServer(&fakeTools{specs: []ai.ToolSpec{readTool()}})

	const workers = 50

	var wg sync.WaitGroup

	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			rec := post(t, s, `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"get_user","arguments":{}}}`)
			assert.Equal(t, http.StatusOK, rec.Code)
		}()
	}

	wg.Wait()
}
