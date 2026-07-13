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
	s := newStreamer(ct)

	_, ok := s.Next()
	assert.False(t, ok)
	require.NoError(t, s.Close())
	assert.True(t, ct.closed)
}
