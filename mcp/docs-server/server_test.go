package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServer(t *testing.T) *server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sampleDump))
	}))
	t.Cleanup(srv.Close)

	return newServer(srv.Client(), srv.URL)
}

// call runs one request through the full stdio loop and returns the
// decoded response, which is what a real MCP client sees.
//
// The transport is newline-delimited, so the request is flattened onto a
// single line first — the fixtures below wrap for readability, and a raw
// newline inside one would otherwise be read as two separate messages.
func call(t *testing.T, s *server, request string) response {
	t.Helper()

	// Collapse only the wrap itself (newline plus its indent). Stripping
	// every space would also join words inside a JSON string value, so a
	// two-term query fixture would silently test one nonsense term.
	line := regexp.MustCompile(`\n\s*`).ReplaceAllString(request, "")

	var out bytes.Buffer

	require.NoError(t, s.serve(context.Background(), strings.NewReader(line+"\n"), &out))

	var resp response
	require.NoError(t, json.Unmarshal(out.Bytes(), &resp))

	return resp
}

func resultText(t *testing.T, resp response) string {
	t.Helper()

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	require.NoError(t, json.Unmarshal(raw, &payload))
	require.NotEmpty(t, payload.Content)

	return payload.Content[0].Text
}

func isToolError(t *testing.T, resp response) bool {
	t.Helper()

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	var payload struct {
		IsError bool `json:"isError"`
	}

	require.NoError(t, json.Unmarshal(raw, &payload))

	return payload.IsError
}

func TestInitialize(t *testing.T) {
	resp := call(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	require.Nil(t, resp.Error)

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	assert.Contains(t, string(raw), protocolVer)
	assert.Contains(t, string(raw), serverName)
}

func TestToolsList(t *testing.T) {
	resp := call(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	require.Nil(t, resp.Error)

	raw, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	for _, tool := range []string{"search_docs", "get_doc", "list_sections"} {
		assert.Contains(t, string(raw), tool)
	}
}

func TestToolDefinitionsAreWellFormed(t *testing.T) {
	for _, def := range toolDefinitions() {
		assert.NotEmpty(t, def.Name)
		assert.NotEmpty(t, def.Description,
			"%s needs a description for the model to route on", def.Name)
		assert.Equal(t, schemaTypeObject, def.InputSchema.Type,
			"%s needs an object input schema", def.Name)

		for _, required := range def.InputSchema.Required {
			assert.Contains(t, def.InputSchema.Properties, required,
				"%s marks %q required but does not declare it", def.Name, required)
		}
	}
}

func TestPing(t *testing.T) {
	resp := call(t, testServer(t), `{"jsonrpc":"2.0","id":9,"method":"ping"}`)

	require.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestSearchDocsTool(t *testing.T) {
	s := testServer(t)

	t.Run("returns ranked results", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call",
			"params":{"name":"search_docs","arguments":{"query":"kafka"}}}`)

		require.Nil(t, resp.Error)

		text := resultText(t, resp)

		assert.Contains(t, text, "https://gofr.dev/")
		assert.False(t, isToolError(t, resp))
	})

	t.Run("no match is a tool error, not a protocol error", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/call",
			"params":{"name":"search_docs","arguments":{"query":"zzzznope"}}}`)

		require.Nil(t, resp.Error)
		assert.True(t, isToolError(t, resp))
		assert.Contains(t, resultText(t, resp), "No GoFr documentation matched")
	})

	t.Run("empty query is rejected", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call",
			"params":{"name":"search_docs","arguments":{"query":"  "}}}`)

		assert.True(t, isToolError(t, resp))
	})

	t.Run("multi-term query requires every term", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":5,"method":"tools/call",
			"params":{"name":"search_docs","arguments":{"query":"kafka zzzznope"}}}`)

		assert.True(t, isToolError(t, resp))
	})

	t.Run("limit is honored", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":4,"method":"tools/call",
			"params":{"name":"search_docs","arguments":{"query":"gofr","limit":1}}}`)

		assert.Contains(t, resultText(t, resp), "1 result(s)")
	})
}

func TestGetDocTool(t *testing.T) {
	s := testServer(t)

	t.Run("returns the full page", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call",
			"params":{"name":"get_doc","arguments":{"path":"/docs/quick-start/introduction"}}}`)

		require.Nil(t, resp.Error)
		assert.Contains(t, resultText(t, resp), "opinionated Go framework")
	})

	t.Run("unknown path is a tool error", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/call",
			"params":{"name":"get_doc","arguments":{"path":"/docs/nope"}}}`)

		assert.True(t, isToolError(t, resp))
	})

	t.Run("missing path is a tool error", func(t *testing.T) {
		resp := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call",
			"params":{"name":"get_doc","arguments":{}}}`)

		assert.True(t, isToolError(t, resp))
	})
}

func TestListSectionsTool(t *testing.T) {
	resp := call(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"tools/call",
		"params":{"name":"list_sections","arguments":{}}}`)

	require.Nil(t, resp.Error)
	assert.Contains(t, resultText(t, resp), "/docs/quick-start")
}

func TestProtocolErrors(t *testing.T) {
	s := testServer(t)

	tests := []struct {
		name     string
		line     string
		wantCode int
	}{
		{"malformed json", `{not json`, codeParseError},
		{"missing method", `{"jsonrpc":"2.0","id":1}`, codeInvalidRequest},
		{"unknown method", `{"jsonrpc":"2.0","id":1,"method":"nope"}`, codeMethodNotFound},
		{
			"unknown tool",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
			codeMethodNotFound,
		},
		{
			"invalid tool params",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"}`,
			codeInvalidRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := call(t, s, tc.line)

			require.NotNil(t, resp.Error)
			assert.Equal(t, tc.wantCode, resp.Error.Code)
		})
	}
}

func TestNotificationsAreNotAnswered(t *testing.T) {
	var out bytes.Buffer

	// No id => a notification. Replying to one makes strict clients drop
	// the connection, so the server must stay silent.
	err := testServer(t).serve(context.Background(),
		strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), &out)

	require.NoError(t, err)
	assert.Empty(t, out.String())
}

func TestBlankLinesAreSkipped(t *testing.T) {
	var out bytes.Buffer

	err := testServer(t).serve(context.Background(),
		strings.NewReader("\n   \n"+`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), &out)

	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(strings.TrimSpace(out.String()), "\n")+1)
}

func TestUnreachableCorpusIsAnInternalError(t *testing.T) {
	s := newServer(http.DefaultClient, "http://127.0.0.1:1/llms-full.txt")

	resp := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call",
		"params":{"name":"list_sections","arguments":{}}}`)

	require.NotNil(t, resp.Error)
	assert.Equal(t, codeInternalError, resp.Error.Code)
}
