package llm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func embedServer(t *testing.T, status int, body string, gotBody *string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)

		if gotBody != nil {
			b, _ := io.ReadAll(r.Body)
			*gotBody = string(b)
		}

		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func TestClient_Embed_Success(t *testing.T) {
	body := `{"model":"m","data":[` +
		`{"embedding":[0.1,0.2],"index":0},` +
		`{"embedding":[0.3,0.4],"index":1}],` +
		`"usage":{"prompt_tokens":5}}`

	var reqBody string

	srv := embedServer(t, http.StatusOK, body, &reqBody)
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	resp, err := c.Embed(t.Context(), []string{"hello", "world"})
	require.NoError(t, err)
	require.Len(t, resp.Embeddings, 2)
	assert.Equal(t, []float32{0.1, 0.2}, resp.Embeddings[0])
	assert.Equal(t, []float32{0.3, 0.4}, resp.Embeddings[1])
	assert.Equal(t, "m", resp.Model)
	assert.Equal(t, 5, resp.Usage.PromptTokens)
	// The request carries the configured model and the inputs, in order.
	assert.JSONEq(t, `{"model":"test-model","input":["hello","world"]}`, reqBody)
}

func TestClient_Embed_ProviderErrorInBody(t *testing.T) {
	srv := embedServer(t, http.StatusOK, `{"error":{"message":"bad input"}}`, nil)
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, errProvider)
}

func TestClient_Embed_MalformedJSON(t *testing.T) {
	srv := embedServer(t, http.StatusOK, `{not json`, nil)
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, errDecodeResponse)
}

func TestClient_Embed_StatusError(t *testing.T) {
	srv := embedServer(t, http.StatusBadRequest, `{"error":{"message":"nope"}}`, nil)
	defer srv.Close()

	c := testClient(t, OpenAI, srv.URL)

	_, err := c.Embed(t.Context(), []string{"x"})
	require.Error(t, err)
}

func TestClient_Embed_NotConnected(t *testing.T) {
	c := &Client{Provider: OpenAI, Model: "m"}

	_, err := c.Embed(t.Context(), []string{"x"})
	require.ErrorIs(t, err, errNotConnected)
}
