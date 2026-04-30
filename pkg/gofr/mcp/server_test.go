package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeApp is a tiny stand-in for the GoFr framework wiring: a mux
// router with a handful of routes, plus a learner that has had a few
// schemas pre-recorded so we can verify the bridge & manifest end-to-end.
func fakeApp() (*mux.Router, *Learner) {
	r := mux.NewRouter()

	type user struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	type createUserBody struct {
		Name string `json:"name"`
	}

	// GET /users — returns a list, no path params.
	r.NewRoute().Methods(http.MethodGet).Path("/users").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]user{{ID: "1", Name: "Ada"}, {ID: "2", Name: "Linus"}})
	})

	// GET /users/{id} — echoes the path param.
	r.NewRoute().Methods(http.MethodGet).Path("/users/{id}").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user{ID: vars["id"], Name: "from-bridge"})
	})

	// POST /users — accepts a JSON body.
	r.NewRoute().Methods(http.MethodPost).Path("/users").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var b createUserBody
		_ = json.NewDecoder(req.Body).Decode(&b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(user{ID: "99", Name: b.Name})
	})

	// Auth-checking route — used to verify auth headers are forwarded.
	r.NewRoute().Methods(http.MethodGet).Path("/secret").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer good-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	l := NewLearner("")
	// Pretend the routes have already seen traffic. In real life the
	// learner is fed by the request wrapper at handler time.
	l.Register(http.MethodGet, "/users", nil)
	l.Register(http.MethodGet, "/users/{id}", []string{"id"})
	l.Register(http.MethodPost, "/users", nil)
	l.Register(http.MethodGet, "/secret", nil)

	return r, l
}

func doRPC(t *testing.T, server *Server, body any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "response body: %s", rec.Body.String())

	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	return out
}

func TestServer_Initialize(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test-app", "1.2.3")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"clientInfo":      map[string]any{"name": "test", "version": "0"},
		},
	})

	result, _ := resp["result"].(map[string]any)
	assert.Equal(t, "2025-06-18", result["protocolVersion"])

	srv, _ := result["serverInfo"].(map[string]any)
	assert.Equal(t, "test-app", srv["name"])
	assert.Equal(t, "1.2.3", srv["version"])

	caps, _ := result["capabilities"].(map[string]any)
	assert.Contains(t, caps, "tools")
}

func TestServer_Initialize_UnknownVersion_FallsBackToLatest(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": "1999-01-01"},
	})

	result, _ := resp["result"].(map[string]any)
	assert.Equal(t, "2025-06-18", result["protocolVersion"])
}

func TestServer_ToolsList_GETOnlyByDefault(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})

	result, _ := resp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)

	names := []string{}
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		names = append(names, tool["name"].(string))
	}

	assert.Contains(t, names, "get_users")
	assert.Contains(t, names, "get_users_id")
	assert.NotContains(t, names, "post_users", "mutations should be hidden by default")
}

func TestServer_ToolsList_AllowMutations(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{AllowMutations: true}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/list",
	})

	result, _ := resp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)

	names := []string{}
	for _, item := range tools {
		names = append(names, item.(map[string]any)["name"].(string))
	}

	assert.Contains(t, names, "post_users")
}

func TestServer_ToolsCall_GET_WithPathParam(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_users_id",
			"arguments": map[string]any{"id": "42"},
		},
	})

	result, _ := resp["result"].(map[string]any)
	assertNotError(t, result)

	structured, _ := result["structuredContent"].(map[string]any)
	assert.Equal(t, "42", structured["id"])
	assert.Equal(t, "from-bridge", structured["name"])
}

// assertNotError tolerates the omitempty zero-value form (key absent).
func assertNotError(t *testing.T, result map[string]any) {
	t.Helper()
	v, ok := result["isError"]
	if !ok {
		return
	}
	assert.Equal(t, false, v, "tool call returned error: %v", result)
}

func TestServer_ToolsCall_POST_WithBody(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{AllowMutations: true}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "post_users",
			"arguments": map[string]any{
				"body": map[string]any{"name": "Grace"},
			},
		},
	})

	result, _ := resp["result"].(map[string]any)
	assertNotError(t, result)

	structured, _ := result["structuredContent"].(map[string]any)
	assert.Equal(t, "99", structured["id"])
	assert.Equal(t, "Grace", structured["name"])
}

func TestServer_ToolsCall_UnknownTool_IsError(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params":  map[string]any{"name": "no_such_tool"},
	})

	result, _ := resp["result"].(map[string]any)
	assert.Equal(t, true, result["isError"])
}

func TestServer_ToolsCall_ForwardsAuthHeaders(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params":  map[string]any{"name": "get_secret"},
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good-token")

	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	result, _ := resp["result"].(map[string]any)
	assertNotError(t, result)
}

func TestServer_ToolsCall_MissingPathParam_IsError(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_users_id",
			"arguments": map[string]any{}, // missing "id"
		},
	})

	result, _ := resp["result"].(map[string]any)
	assert.Equal(t, true, result["isError"])
	content, _ := result["content"].([]any)
	require.NotEmpty(t, content)
	first, _ := content[0].(map[string]any)
	assert.Contains(t, strings.ToLower(first["text"].(string)), "id")
}

func TestServer_RejectsNonPOST(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestServer_Notification_NoResponse(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		// No id — this is a notification per JSON-RPC.
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestServer_UnknownMethod_RespondsWithError(t *testing.T) {
	r, l := fakeApp()
	server := NewServer(r, l, BuildOptions{}, "test", "1")

	resp := doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "resources/list",
	})

	rpcErr, _ := resp["error"].(map[string]any)
	require.NotNil(t, rpcErr, "expected rpc error for unknown method")
	assert.Equal(t, float64(errCodeMethodNotFound), rpcErr["code"])
}
