package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	os.Exit(m.Run())
}

// mockProvider is an in-test OpenAI-compatible server: it answers /models (health) and /embeddings,
// returns a streamed reply when the request asks for one, drives one agent turn (tool call then
// final answer) when tools are present, and otherwise returns a plain completion.
func mockProvider(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}

		if r.URL.Path == "/embeddings" {
			_, _ = io.WriteString(w, embeddingsJSON())
			return
		}

		body, _ := io.ReadAll(r.Body)

		switch {
		case bytes.Contains(body, []byte(`"stream":true`)):
			writeSSE(w)
		case bytes.Contains(body, []byte(`"role":"tool"`)):
			_, _ = io.WriteString(w, chatJSON("final answer: 12 in stock"))
		case bytes.Contains(body, []byte(`"tools":`)):
			_, _ = io.WriteString(w, toolCallJSON())
		default:
			_, _ = io.WriteString(w, chatJSON("a concise summary"))
		}
	}))
}

func writeSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")

	flusher, _ := w.(http.Flusher)
	for _, chunk := range []string{"hello ", "world"} {
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", chunk)

		if flusher != nil {
			flusher.Flush()
		}
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func chatJSON(content string) string {
	return fmt.Sprintf(
		`{"model":"mock","choices":[{"message":{"role":"assistant","content":%q}}],`+
			`"usage":{"prompt_tokens":5,"completion_tokens":3}}`, content)
}

// embeddingsJSON answers with the two vectors deliberately out of order — index 1 before index 0.
// The wire contract permits any order, which is exactly what the "index" field is for, so this is a
// response a real OpenAI-compatible backend is allowed to send. The handler must still hand each
// input the vector computed from it, which is what the assertion below pins.
func embeddingsJSON() string {
	return `{"model":"mock","data":[` +
		`{"embedding":[0.3,0.4],"index":1},` +
		`{"embedding":[0.1,0.2],"index":0}],` +
		`"usage":{"prompt_tokens":5}}`
}

func toolCallJSON() string {
	return `{"model":"mock","choices":[{"message":{"role":"assistant","tool_calls":` +
		`[{"id":"c1","type":"function","function":{"name":"get_inventory_sku",` +
		`"arguments":"{\"sku\":\"ABC\"}"}}]}}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`
}

func TestIntegration_UsingAI(t *testing.T) {
	mock := mockProvider(t)
	defer mock.Close()

	httpPort := testutil.GetFreePort(t)

	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(testutil.GetFreePort(t)))
	t.Setenv("MCP_PORT", strconv.Itoa(testutil.GetFreePort(t)))
	t.Setenv("LLM_BASE_URL", mock.URL)
	t.Setenv("LLM_API_KEY", "test-key")

	go main()

	base := fmt.Sprintf("http://localhost:%d", httpPort)
	requireReady(t, base)

	t.Run("inventory handler", func(t *testing.T) {
		assert.Contains(t, get(t, base+"/inventory/ABC"), "ABC")
	})

	t.Run("health reports only the redacted aggregate status", func(t *testing.T) {
		out := get(t, base+"/.well-known/health")
		assert.Contains(t, out, `"status":"UP"`)
		// The unauthenticated endpoint must expose only {name, status} — never per-dependency
		// details such as the datasource name, provider, or credentials.
		assert.NotContains(t, out, `"llm"`)
		assert.NotContains(t, out, `"provider":"groq"`)
		assert.NotContains(t, out, "test-key", "the API key must never appear in health details")
	})

	t.Run("ask calls the LLM", func(t *testing.T) {
		assert.Contains(t, post(t, base+"/ask", `{"prompt":"hi"}`), "a concise summary")
	})

	t.Run("stream returns SSE tokens then done", func(t *testing.T) {
		out := post(t, base+"/stream", `{"prompt":"hi"}`)
		assert.Contains(t, out, "hello")
		assert.Contains(t, out, "world")
		assert.Contains(t, out, "[DONE]")
	})

	t.Run("agent runs a tool loop", func(t *testing.T) {
		assert.Contains(t, post(t, base+"/agent", `{"task":"stock?"}`), "final answer")
	})

	t.Run("embed returns one vector per input, in input order", func(t *testing.T) {
		out := post(t, base+"/embed", `{"inputs":["the quick brown fox","a fast auburn fox"]}`)
		// The provider answered index 1 first. Pairing by array position would return these
		// swapped, and nothing downstream would notice — a wrong vector is still a valid vector.
		assert.Contains(t, out, `[[0.1,0.2],[0.3,0.4]]`)
	})

	t.Run("no goroutine leak under repeated streaming", func(t *testing.T) {
		assertNoStreamLeak(t, base)
	})
}

// assertNoStreamLeak runs many streaming requests and asserts the goroutine count returns to the
// warm baseline, so the per-request stream reader goroutines are all released.
func assertNoStreamLeak(t *testing.T, base string) {
	t.Helper()

	post(t, base+"/stream", `{"prompt":"warmup"}`) // warm up connections
	http.DefaultClient.CloseIdleConnections()
	runtime.GC()

	baseline := runtime.NumGoroutine()

	for range 50 {
		post(t, base+"/stream", `{"prompt":"x"}`)
	}

	http.DefaultClient.CloseIdleConnections()
	runtime.GC()

	for range 100 {
		if runtime.NumGoroutine() <= baseline+2 {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("goroutines did not settle: baseline %d, now %d", baseline, runtime.NumGoroutine())
}

func requireReady(t *testing.T, base string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/.well-known/alive") //nolint:noctx // test readiness probe
		if err == nil {
			_ = resp.Body.Close()
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("server did not start")
}

func get(t *testing.T, url string) string {
	t.Helper()

	resp, err := http.Get(url) //nolint:noctx // integration test
	require.NoError(t, err)

	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return string(b)
}

func post(t *testing.T, url, body string) string {
	t.Helper()

	resp, err := http.Post(url, "application/json", strings.NewReader(body)) //nolint:noctx // integration test
	require.NoError(t, err)

	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)

	return string(b)
}
