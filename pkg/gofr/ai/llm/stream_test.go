package llm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
)

func sseServer(t *testing.T, status int, lines []string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)

		flusher, _ := w.(http.Flusher)

		for _, l := range lines {
			_, _ = io.WriteString(w, l+"\n")

			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
}

func collect(t *testing.T, s ai.Streamer) []string {
	t.Helper()

	var out []string

	for {
		v, ok := s.Next()
		if !ok {
			break
		}

		out = append(out, v.(string))
	}

	return out
}

func TestClient_Stream_Success(t *testing.T) {
	lines := []string{
		`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
		``,
		`: keep-alive comment`,
		`data: {"choices":[{"delta":{"content":" world"}}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
		`data: [DONE]`,
		`data: {"choices":[{"delta":{"content":"after done"}}]}`,
	}
	srv := sseServer(t, http.StatusOK, lines)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)

	defer func() { require.NoError(t, s.Close()) }()

	got := collect(t, s)
	require.NoError(t, s.Err())
	assert.Equal(t, []string{"Hello", " world"}, got)

	u, ok := s.(interface{ Usage() ai.Usage })
	require.True(t, ok)
	assert.Equal(t, ai.Usage{PromptTokens: 5, CompletionTokens: 2}, u.Usage())
}

// A trailing chunk carrying "usage": null (sent by some gateways after the finish chunk) must not
// wipe the usage captured earlier.
func TestClient_Stream_NullUsageDoesNotWipe(t *testing.T) {
	lines := []string{
		`data: {"choices":[{"delta":{"content":"hi"}}],"usage":null}`,
		`data: {"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4}}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":null}`,
		`data: [DONE]`,
	}
	srv := sseServer(t, http.StatusOK, lines)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)

	defer func() { require.NoError(t, s.Close()) }()

	collect(t, s)
	require.NoError(t, s.Err())

	u := s.(interface{ Usage() ai.Usage })
	assert.Equal(t, ai.Usage{PromptTokens: 9, CompletionTokens: 4}, u.Usage())
}

// Custom UsageFields also apply on the streaming path, mapped from the final chunk's usage object.
func TestClient_Stream_CustomUsageFields(t *testing.T) {
	lines := []string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":100,"usage_metadata":{"cached_content_token_count":64}}}`,
		`data: [DONE]`,
	}
	srv := sseServer(t, http.StatusOK, lines)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)
	c.UsageFields = UsageFields{CachedTokens: "usage_metadata.cached_content_token_count"}

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)

	defer func() { require.NoError(t, s.Close()) }()

	collect(t, s)
	require.NoError(t, s.Err())

	u := s.(interface{ Usage() ai.Usage })
	assert.Equal(t, 100, u.Usage().PromptTokens)
	assert.Equal(t, 64, u.Usage().CachedTokens)
}

func TestClient_Stream_MalformedChunk(t *testing.T) {
	srv := sseServer(t, http.StatusOK, []string{`data: {broken`})

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)

	defer func() { _ = s.Close() }()

	_, ok := s.Next()
	assert.False(t, ok)
	require.ErrorIs(t, s.Err(), errStreamRead)
}

func TestClient_Stream_OversizedLine(t *testing.T) {
	huge := "data: " + strings.Repeat("a", streamBufferMax+1)
	srv := sseServer(t, http.StatusOK, []string{huge})

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)

	defer func() { _ = s.Close() }()

	_, ok := s.Next()
	assert.False(t, ok)
	require.ErrorIs(t, s.Err(), errStreamRead)
}

func TestClient_Stream_StatusError(t *testing.T) {
	srv := sseServer(t, http.StatusUnauthorized, []string{`{"error":"nope"}`})

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)
	c.apiKey = "leak-me-not"

	_, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errUnexpectedStatus)
	assert.NotContains(t, err.Error(), "leak-me-not")
}

func TestClient_Stream_NotConnected(t *testing.T) {
	c := &Client{Provider: OpenAI, Model: "m"}

	_, err := c.Stream(t.Context(), nil)
	require.ErrorIs(t, err, errNotConnected)
}

type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error {
	c.closed = true

	return nil
}

func TestStreamer_CloseClosesBody(t *testing.T) {
	ct := &closeTracker{Reader: strings.NewReader(`data: [DONE]` + "\n")}
	s := newStreamer(ct, UsageFields{})

	_, ok := s.Next()
	assert.False(t, ok)
	require.NoError(t, s.Close())
	assert.True(t, ct.closed)
}

// Tool calls streamed as deltas (name in one chunk, arguments fragmented across several) must be
// assembled and returned via ToolCalls after the stream is drained.
func TestClient_Stream_AssemblesFragmentedToolCall(t *testing.T) {
	calls := drainToolCalls(t, []string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"search"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"gofr\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	})

	require.Len(t, calls, 1)
	assert.Equal(t, "c1", calls[0].ID)
	assert.Equal(t, "search", calls[0].Name)
	assert.JSONEq(t, `{"q":"gofr"}`, string(calls[0].Args))
}

// A stream truncated mid-arguments must not yield invalid JSON: the assembled args normalize to {}.
func TestClient_Stream_TruncatedToolArgsNormalized(t *testing.T) {
	calls := drainToolCalls(t, []string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"search","arguments":"{\"q\":"}}]}}]}`,
		`data: [DONE]`,
	})

	require.Len(t, calls, 1)
	assert.JSONEq(t, `{}`, string(calls[0].Args))
}

func TestClient_Stream_MultipleToolCallsByIndex(t *testing.T) {
	calls := drainToolCalls(t, []string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"one","arguments":"{}"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","function":{"name":"two","arguments":"{}"}}]}}]}`,
		`data: [DONE]`,
	})

	require.Len(t, calls, 2)
	assert.Equal(t, "one", calls[0].Name)
	assert.Equal(t, "two", calls[1].Name)
}

func TestClient_Stream_EmptyToolArgsNormalized(t *testing.T) {
	calls := drainToolCalls(t, []string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"noargs"}}]}}]}`,
		`data: [DONE]`,
	})

	require.Len(t, calls, 1)
	assert.JSONEq(t, `{}`, string(calls[0].Args))
}

func TestClient_Stream_ContentThenToolCall(t *testing.T) {
	srv := sseServer(t, http.StatusOK, []string{
		`data: {"choices":[{"delta":{"content":"thinking"}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"go","arguments":"{}"}}]}}]}`,
		`data: [DONE]`,
	})
	defer srv.Close()

	s, err := testClient(t, OpenAI, srv.URL).Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "x"}})
	require.NoError(t, err)

	defer s.Close()

	assert.Equal(t, []string{"thinking"}, collect(t, s))
	require.NoError(t, s.Err())

	tc, ok := s.(ai.ToolCallStreamer)
	require.True(t, ok)
	require.Len(t, tc.ToolCalls(), 1)
	assert.Equal(t, "go", tc.ToolCalls()[0].Name)
}

func drainToolCalls(t *testing.T, lines []string) []ai.ToolCall {
	t.Helper()

	srv := sseServer(t, http.StatusOK, lines)
	defer srv.Close()

	s, err := testClient(t, OpenAI, srv.URL).Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "x"}})
	require.NoError(t, err)

	defer s.Close()

	collect(t, s) // drain content

	require.NoError(t, s.Err())

	tc, ok := s.(ai.ToolCallStreamer)
	require.True(t, ok)

	return tc.ToolCalls()
}

func TestClient_Stream_ErrorChunkSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "data: {\"error\":{\"message\":\"boom\"}}\n\n")
	}))
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)

	defer s.Close()

	_, ok := s.Next()
	assert.False(t, ok)
	require.ErrorIs(t, s.Err(), errProvider)
}

func TestClient_Stream_CRLFLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\r\n\r\ndata: [DONE]\r\n\r\n")
	}))
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	s, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "x"}})
	require.NoError(t, err)

	defer s.Close()

	v, ok := s.Next()
	require.True(t, ok)
	assert.Equal(t, "hi", v)
	require.NoError(t, s.Err())
}
