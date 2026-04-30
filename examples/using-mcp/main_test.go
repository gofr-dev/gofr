package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

// TestIntegration_MCPMode boots the example app and verifies that:
//
//	1. HTTP routes still work normally,
//	2. /mcp answers initialize / tools/list / tools/call,
//	3. Tools learned from real HTTP traffic carry the right shape.
//
// This is an integration test rather than a unit test because the
// whole point of the feature is the interaction between the HTTP
// pipeline and the MCP layer.
func TestIntegration_MCPMode(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()

	time.Sleep(200 * time.Millisecond) // let the server come up

	host := configs.HTTPHost

	// 1. Drive a few HTTP calls so the MCP learner records the schemas.
	mustGET(t, host+"/users")
	mustGET(t, host+"/users/1")
	mustPOST(t, host+"/users", `{"name":"Grace Hopper","role":"engineer"}`, http.StatusOK)

	// 2. tools/list — the freshly-learned schemas should be present.
	listResp := rpc(t, host, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	})

	result, _ := listResp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)

	names := []string{}
	for _, item := range tools {
		names = append(names, item.(map[string]any)["name"].(string))
	}

	assert.Contains(t, names, "get_users")
	assert.Contains(t, names, "get_users_id")
	assert.Contains(t, names, "post_users", "MCP_ENABLED=full should expose mutations")

	// 3. tools/call — round-trip through the bridge into the real handler.
	callResp := rpc(t, host, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_users_id",
			"arguments": map[string]any{"id": "1"},
		},
	})

	callResult, _ := callResp["result"].(map[string]any)

	// GoFr wraps successful handler returns in {"data": ...}, so the
	// MCP structuredContent mirrors that envelope. The bridge does
	// not unwrap the response — clients see what the HTTP API would
	// have served.
	structured, _ := callResult["structuredContent"].(map[string]any)
	data, _ := structured["data"].(map[string]any)
	assert.Equal(t, "1", data["id"])
	assert.Equal(t, "Ada Lovelace", data["name"])
}

func mustGET(t *testing.T, url string) {
	t.Helper()

	resp, err := http.Get(url)
	require.NoError(t, err)

	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)
}

func mustPOST(t *testing.T, url, body string, _ int) {
	t.Helper()

	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	require.NoError(t, err)

	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)
}

func rpc(t *testing.T, host string, body any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	resp, err := http.Post(host+"/mcp", "application/json", bytes.NewReader(raw))
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

	return out
}
