package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/ai"
)

// limitedClient builds a connected client with a concurrency cap (testClient connects before the
// field could be set, so construct directly).
func limitedClient(baseURL string, limit int) *Client {
	c := &Client{Provider: OpenAI, Model: "test-model", BaseURL: baseURL, MaxConcurrentRequests: limit}
	c.UseLogger(nopLogger{})
	c.UseMetrics(nil)
	c.Connect()

	return c
}

// countingChatServer records the peak number of simultaneously in-flight requests.
func countingChatServer(inflight, peak *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := inflight.Add(1)

		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}

		time.Sleep(40 * time.Millisecond) // hold the slot so concurrent calls overlap
		inflight.Add(-1)

		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
}

func TestClient_MaxConcurrentRequests_CapsInFlight(t *testing.T) {
	const (
		limit   = 2
		callers = 8
	)

	var inflight, peak atomic.Int32

	srv := countingChatServer(&inflight, &peak)
	defer srv.Close()

	c := limitedClient(srv.URL, limit)

	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, _ = c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
		}()
	}

	wg.Wait()

	assert.LessOrEqual(t, int(peak.Load()), limit, "in-flight requests must never exceed the limit")
	assert.EqualValues(t, limit, peak.Load(), "concurrent load should saturate the limit")
}

func TestClient_MaxConcurrentRequests_Unlimited(t *testing.T) {
	const callers = 6

	var inflight, peak atomic.Int32

	srv := countingChatServer(&inflight, &peak)
	defer srv.Close()

	c := limitedClient(srv.URL, 0) // 0 = no cap

	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, _ = c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hi"}})
		}()
	}

	wg.Wait()

	assert.Greater(t, int(peak.Load()), 1, "with no cap, requests should run concurrently")
}

// A caller waiting for a slot honors context cancellation instead of blocking forever.
func TestClient_MaxConcurrentRequests_ContextCancelledWhileWaiting(t *testing.T) {
	var inflight, peak atomic.Int32

	srv := countingChatServer(&inflight, &peak)
	defer srv.Close()

	c := limitedClient(srv.URL, 1)

	// Fill the only slot with a long-running call.
	started := make(chan struct{})

	go func() {
		close(started)

		_, _ = c.Chat(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "hold"}})
	}()

	<-started
	time.Sleep(10 * time.Millisecond) // let the first call take the slot

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()

	_, err := c.Chat(ctx, []ai.Message{{Role: ai.RoleUser, Content: "waits"}})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// A stream must free its in-flight slot when drained to exhaustion, even if the caller never Close()s.
func TestClient_MaxConcurrentRequests_StreamReleasesOnExhaustion(t *testing.T) {
	srv := sseServer(t, http.StatusOK, []string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: [DONE]`,
	})
	defer srv.Close()

	c := limitedClient(srv.URL, 1)

	s1, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "a"}})
	require.NoError(t, err)

	for {
		if _, ok := s1.Next(); !ok { // drain to exhaustion; deliberately no Close
			break
		}
	}

	// With the only slot freed by exhaustion, a second stream must acquire it within a short deadline.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	s2, err := c.Stream(ctx, []ai.Message{{Role: ai.RoleUser, Content: "b"}})
	require.NoError(t, err, "exhausting the first stream should have freed the slot")

	_ = s2.Close()
}

// Closing a stream frees its slot even if it was not consumed.
func TestClient_MaxConcurrentRequests_StreamReleasesOnClose(t *testing.T) {
	srv := sseServer(t, http.StatusOK, []string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: [DONE]`,
	})
	defer srv.Close()

	c := limitedClient(srv.URL, 1)

	s1, err := c.Stream(t.Context(), []ai.Message{{Role: ai.RoleUser, Content: "a"}})
	require.NoError(t, err)
	require.NoError(t, s1.Close()) // release without consuming

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	s2, err := c.Stream(ctx, []ai.Message{{Role: ai.RoleUser, Content: "b"}})
	require.NoError(t, err, "closing the first stream should have freed the slot")

	_ = s2.Close()
}
