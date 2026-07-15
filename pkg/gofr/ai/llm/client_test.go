package llm

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/datasource"
)

type nopLogger struct{}

func (nopLogger) Log(...any) {}

type envConfig struct{}

func (envConfig) Get(k string) string { return os.Getenv(k) }

func (envConfig) GetOrDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}

	return d
}

func testClient(t *testing.T, provider Provider, baseURL string) *Client {
	t.Helper()

	c := &Client{Provider: provider, Model: "test-model", BaseURL: baseURL}
	c.UseLogger(nopLogger{})
	c.UseMetrics(nil)
	c.Connect()

	return c
}

func chatServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func TestClient_Chat_Success(t *testing.T) {
	body := `{"model":"m","choices":[{"message":{"content":"hi there"}}],` +
		`"usage":{"prompt_tokens":7,"completion_tokens":3}}`
	srv := chatServer(t, http.StatusOK, body)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	resp, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)
	assert.Equal(t, "hi there", resp.Content)
	assert.Equal(t, "m", resp.Model)
	assert.Equal(t, 7, resp.Usage.PromptTokens)
	assert.Equal(t, 3, resp.Usage.CompletionTokens)
}

func TestClient_Chat_ToolCalls(t *testing.T) {
	body := `{"choices":[{"message":{"content":"","tool_calls":` +
		`[{"id":"call_1","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]}}]}`
	srv := chatServer(t, http.StatusOK, body)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	resp, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call_1", resp.ToolCalls[0].ID)
	assert.Equal(t, "lookup", resp.ToolCalls[0].Name)
	assert.JSONEq(t, `{"q":"x"}`, string(resp.ToolCalls[0].Args))
}

// A custom provider whose usage uses non-standard field names is mapped end-to-end via UsageFields.
func TestClient_Chat_CustomUsageFields(t *testing.T) {
	body := `{"model":"m","choices":[{"message":{"content":"ok"}}],` +
		`"usage":{"prompt_tokens":5000,"completion_tokens":400,"total_tokens":5400,` +
		`"usage_metadata":{"cached_content_token_count":4096,"thoughts_token_count":210}}}`
	srv := chatServer(t, http.StatusOK, body)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)
	c.UsageFields = UsageFields{
		CachedTokens:    "usage_metadata.cached_content_token_count",
		ReasoningTokens: "usage_metadata.thoughts_token_count",
	}

	resp, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)
	assert.Equal(t, 5000, resp.Usage.PromptTokens)
	assert.Equal(t, 400, resp.Usage.CompletionTokens)
	assert.Equal(t, 5400, resp.Usage.TotalTokens)
	assert.Equal(t, 4096, resp.Usage.CachedTokens)
	assert.Equal(t, 210, resp.Usage.ReasoningTokens)
}

// Streaming requests must ask for the final usage chunk; non-streaming requests must not.
func TestClient_BuildRequest_StreamRequestsUsage(t *testing.T) {
	c := &Client{Model: "m"}

	streamed, err := c.buildRequest([]ai.Message{{Role: ai.RoleUser, Content: "hi"}}, nil, true)
	require.NoError(t, err)
	assert.Contains(t, string(streamed), `"stream_options":{"include_usage":true}`)

	plain, err := c.buildRequest([]ai.Message{{Role: ai.RoleUser, Content: "hi"}}, nil, false)
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "stream_options")
}

func TestClient_Chat_MissingUsage(t *testing.T) {
	srv := chatServer(t, http.StatusOK, `{"choices":[{"message":{"content":"ok"}}]}`)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	resp, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, 0, resp.Usage.PromptTokens)
	assert.Equal(t, 0, resp.Usage.CompletionTokens)
}

func TestClient_Chat_MalformedJSON(t *testing.T) {
	srv := chatServer(t, http.StatusOK, `{not json`)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errDecodeResponse)
}

func TestClient_Chat_AuthErrorHidesKey(t *testing.T) {
	const secret = "sk-super-secret-key-should-not-leak"

	srv := chatServer(t, http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`)

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)
	c.apiKey = secret

	_, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errUnexpectedStatus)
	assert.NotContains(t, err.Error(), secret)
}

func TestClient_Chat_403NotRetried(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusForbidden)
	}))

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errUnexpectedStatus)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count))
}

func TestClient_Chat_429RetriedThenSuccess(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&count, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"recovered"}}]}`)
	}))

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	resp, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.NoError(t, err)
	assert.Equal(t, "recovered", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&count))
}

func TestClient_Chat_429ExhaustedReturnsStatusError(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
	require.ErrorIs(t, err, errUnexpectedStatus)
	assert.Equal(t, int32(maxRetries+1), atomic.LoadInt32(&count))
}

func TestClient_Chat_ContextCancelNotRetried(t *testing.T) {
	var count int32

	received := make(chan struct{}, 1)
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)

		select {
		case received <- struct{}{}:
		default:
		}

		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))

	defer srv.Close()
	defer close(release)

	c := testClient(t, OpenAI, srv.URL)
	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)

	go func() {
		_, err := c.Chat(ctx, []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
		errCh <- err
	}()

	<-received
	cancel()

	err := <-errCh
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count))
}

func TestClient_Chat_NotConnected(t *testing.T) {
	c := &Client{Provider: OpenAI, Model: "m"}

	_, err := c.Chat(t.Context(), nil)
	require.ErrorIs(t, err, errNotConnected)
}

func TestClient_HealthCheck(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusOK)
	}))

	defer srv.Close()

	c := testClient(t, Groq, srv.URL)
	c.apiKey = "secret-health-key"

	h := c.HealthCheck(t.Context())
	assert.Equal(t, datasource.StatusUp, h.Status)
	assert.Equal(t, "groq", h.Details["provider"])
	assert.Equal(t, "test-model", h.Details["model"])
	assert.NotContains(t, h.Details, "api_key")

	for _, v := range h.Details {
		assert.NotContains(t, strconv.Quote(toString(v)), "secret-health-key")
	}

	// Second call is served from the TTL cache and must not hit the server again.
	c.HealthCheck(t.Context())
	assert.Equal(t, int32(1), atomic.LoadInt32(&count))
}

func TestClient_HealthCheck_Down(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	h := c.HealthCheck(t.Context())
	assert.Equal(t, datasource.StatusDown, h.Status)
}

func toString(v any) string {
	s, _ := v.(string)

	return s
}

func TestClient_Names(t *testing.T) {
	c := &Client{Provider: DeepSeek, Model: "deepseek-chat"}

	assert.Equal(t, "deepseek", c.Name())
	assert.Equal(t, "deepseek-chat", c.ModelName())
}

func TestClient_ResolveDefaults(t *testing.T) {
	openaiDefault, _ := providerDefaults(OpenAI)
	groqDefault, _ := providerDefaults(Groq)

	tests := []struct {
		name     string
		provider Provider
		baseURL  string
		envKey   string
		envVal   string
		wantBase string
		wantKey  string
	}{
		{"openai default", OpenAI, "", "OPENAI_API_KEY", "k-openai", openaiDefault.baseURL, "k-openai"},
		{"groq default", Groq, "", "GROQ_API_KEY", "k-groq", groqDefault.baseURL, "k-groq"},
		{"base url override", Together, "http://localhost:9999", "TOGETHER_API_KEY", "k-t", "http://localhost:9999", "k-t"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, tc.envVal)

			c := &Client{Provider: tc.provider, Model: "m", BaseURL: tc.baseURL}
			c.UseConfig(envConfig{})

			assert.Equal(t, tc.wantBase, c.baseURL)
			assert.Equal(t, tc.wantKey, c.apiKey)
		})
	}
}

func TestClient_ResolveUnknownProvider(t *testing.T) {
	c := &Client{Provider: Provider("unknown"), Model: "m"}
	c.UseConfig(envConfig{})

	assert.Empty(t, c.baseURL)
	assert.Empty(t, c.apiKey)
}

func TestRetryDelay_RespectsRetryAfter(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "2")

	assert.Equal(t, 2*1000, int(retryDelay(resp, 0).Milliseconds()))
}

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

func TestClient_ResolveBaseURLFromConfig(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "http://proxy.example/v1")

	c := &Client{Provider: Groq, Model: "m"}
	c.UseConfig(envConfig{})

	assert.Equal(t, "http://proxy.example/v1", c.baseURL)
}

func TestClient_BaseURLFieldWinsOverConfig(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "http://from-env")

	c := &Client{Provider: Groq, Model: "m", BaseURL: "http://from-field"}
	c.UseConfig(envConfig{})

	assert.Equal(t, "http://from-field", c.baseURL)
}
