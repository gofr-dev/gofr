package llm

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/datasource"
)

var (
	errTestGeneric = errors.New("boom")
	errTestNoRoute = errors.New("no route")
	errTestReset   = errors.New("reset")
)

func TestClient_Chat_ProviderErrorInBody(t *testing.T) {
	srv := chatServer(t, http.StatusOK, `{"error":{"message":"quota exceeded"}}`)
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errProvider)
	assert.ErrorContains(t, err, "quota exceeded")
}

func TestClient_Chat_EmptyChoices(t *testing.T) {
	srv := chatServer(t, http.StatusOK, `{"choices":[]}`)
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	resp, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)
	assert.Empty(t, resp.Content)
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

func TestRetryDelay_CapsRetryAfter(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "3600")

	assert.Equal(t, maxRetryAfter, retryDelay(resp, 0))
}

func TestClient_HealthCheck_Concurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_ = c.HealthCheck(t.Context())
		}()
	}

	wg.Wait()
}

type timeoutNetErr struct{}

func (timeoutNetErr) Error() string   { return "i/o timeout" }
func (timeoutNetErr) Timeout() bool   { return true }
func (timeoutNetErr) Temporary() bool { return true }

func TestIsRetriableConnErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout is never retried", timeoutNetErr{}, false},
		{"dial error is retried", &net.OpError{Op: opDial, Err: errTestNoRoute}, true},
		{"post-send read error is not retried", &net.OpError{Op: "read", Err: errTestReset}, false},
		{"connection refused is retried", &url.Error{Op: "Post", URL: "x", Err: syscall.ECONNREFUSED}, true},
		{"unknown error is not retried", errTestGeneric, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isRetriableConnErr(tc.err))
		})
	}
}

func TestClient_StatusError_RedactsKey(t *testing.T) {
	c := &Client{}
	c.apiKey = "sk-secret-123"

	err := c.statusError(http.StatusBadRequest, []byte("invalid auth: Bearer sk-secret-123"))
	assert.NotContains(t, err.Error(), "sk-secret-123")
	assert.Contains(t, err.Error(), "[REDACTED]")
}

func TestCloneHealth_DetailsNotShared(t *testing.T) {
	orig := datasource.Health{Status: datasource.StatusUp, Details: map[string]any{"host": "openai"}}
	clone := cloneHealth(orig)

	clone.Details["host"] = "mutated"
	assert.Equal(t, "openai", orig.Details["host"])
}

func TestClient_ResolveGenericAPIKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "generic-key")

	c := &Client{Provider: Groq, Model: "m"}
	c.UseConfig(envConfig{})

	assert.Equal(t, "generic-key", c.apiKey)
}

func TestClient_ResolveProviderKeyFallback(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "groq-specific") // no LLM_API_KEY set

	c := &Client{Provider: Groq, Model: "m"}
	c.UseConfig(envConfig{})

	assert.Equal(t, "groq-specific", c.apiKey)
}

func TestClient_GenericKeyPreferredOverProviderKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "generic")
	t.Setenv("GROQ_API_KEY", "specific")

	c := &Client{Provider: Groq, Model: "m"}
	c.UseConfig(envConfig{})

	assert.Equal(t, "generic", c.apiKey)
}

func TestClient_Connect_UnknownProviderNoBaseURL(t *testing.T) {
	c := &Client{Provider: Provider("nope")}
	c.UseLogger(nopLogger{})
	c.Connect()

	_, err := c.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errNotConnected)
}
