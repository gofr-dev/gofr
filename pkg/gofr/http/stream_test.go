package http

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

var errClientGone = errors.New("client gone")

// sliceSource yields a fixed set of values then reports done.
type sliceSource struct {
	items  []any
	idx    int
	err    error
	closed bool
}

func (s *sliceSource) Next() (any, bool) {
	if s.idx >= len(s.items) {
		return nil, false
	}

	v := s.items[s.idx]
	s.idx++

	return v, true
}

func (s *sliceSource) Err() error   { return s.err }
func (s *sliceSource) Close() error { s.closed = true; return nil }

func TestStream_SSE(t *testing.T) {
	rec := httptest.NewRecorder()
	src := &sliceSource{items: []any{"a", "b"}}

	NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)

	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "data: \"a\"\n\ndata: \"b\"\n\ndata: [DONE]\n\n", rec.Body.String())
	assert.True(t, src.closed, "source must be closed after streaming")
}

func TestStream_NDJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	src := &sliceSource{items: []any{map[string]int{"n": 1}, map[string]int{"n": 2}}}

	NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src, Format: resTypes.NDJSON}, nil)

	assert.Equal(t, "application/x-ndjson", rec.Header().Get("Content-Type"))
	assert.Equal(t, "{\"n\":1}\n{\"n\":2}\n", rec.Body.String())
	assert.True(t, src.closed)
}

func TestStream_SourceError(t *testing.T) {
	rec := httptest.NewRecorder()
	src := &sliceSource{items: []any{"a"}, err: errClientGone}

	NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)

	assert.Equal(t, "data: \"a\"\n\nevent: error\ndata: {\"error\":\"client gone\"}\n\n", rec.Body.String())
	assert.True(t, src.closed)
}

func TestStream_UnmarshalableValue(t *testing.T) {
	rec := httptest.NewRecorder()
	src := &sliceSource{items: []any{make(chan int)}} // channels cannot be marshaled

	NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)

	assert.Contains(t, rec.Body.String(), "event: error")
	assert.True(t, src.closed)
}

// erroringWriter fails every write to simulate a disconnected client.
type erroringWriter struct{ header http.Header }

func (e *erroringWriter) Header() http.Header     { return e.header }
func (*erroringWriter) Write([]byte) (int, error) { return 0, errClientGone }
func (*erroringWriter) WriteHeader(int)           {}
func (*erroringWriter) Flush()                    {}

// blockingSource blocks in Next until Close is called, modeling a stream waiting on the network.
type blockingSource struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingSource() *blockingSource { return &blockingSource{closed: make(chan struct{})} }

func (b *blockingSource) Next() (any, bool) {
	<-b.closed
	return nil, false
}

func (*blockingSource) Err() error { return nil }

func (b *blockingSource) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

// A client that drops mid-stream must not leave the producer goroutine blocked: the failed write
// tears the stream down, Close unblocks Next, and the reader exits.
func TestStream_ClientDisconnect_NoLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()
	w := &erroringWriter{header: make(http.Header)}
	src := newBlockingSource()

	done := make(chan struct{})

	go func() {
		defer close(done)

		NewResponder(w, http.MethodGet).Respond(resTypes.Stream{Source: src, Heartbeat: 10 * time.Millisecond}, nil)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Respond did not return after client disconnect — stream is stuck")
	}

	select {
	case <-src.closed:
	default:
		t.Fatal("source was not closed on disconnect — producer would leak")
	}

	assertGoroutinesReturn(t, baseline)
}

// assertGoroutinesReturn polls until the goroutine count is back to baseline, proving the reader
// goroutine exited rather than leaking.
func assertGoroutinesReturn(t *testing.T, baseline int) {
	t.Helper()

	for range 100 {
		if runtime.NumGoroutine() <= baseline {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("goroutines did not return to baseline %d (now %d) — stream leaked", baseline, runtime.NumGoroutine())
}

// unwrapOnlyWriter embeds the ResponseWriter interface (so concrete Flush is NOT promoted) and
// exposes Unwrap — exactly the shape of middleware.StatusResponseWriter. It is how we reproduce the
// production writer chain that a plain httptest.Recorder does not.
type unwrapOnlyWriter struct{ http.ResponseWriter }

func (w unwrapOnlyWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Behind GoFr's real writer wrapper, flush is reached only via Unwrap; every frame must still
// arrive. This is the case the recorder-based tests could not see.
func TestStream_ThroughWrappedWriter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		NewResponder(unwrapOnlyWriter{w}, r.Method).
			Respond(resTypes.Stream{Source: &sliceSource{items: []any{"a", "b"}}}, nil)
	}))
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "data: \"a\"\n\ndata: \"b\"\n\ndata: [DONE]\n\n", string(body))
}

func TestStream_NilSource(t *testing.T) {
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() {
		NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: nil}, nil)
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "stream source is nil")
}

// A value that fails to encode mid-stream terminates the stream with one error frame — no further
// values, no trailing [DONE].
func TestStream_MarshalErrorIsTerminal(t *testing.T) {
	rec := httptest.NewRecorder()
	src := &sliceSource{items: []any{"first", make(chan int), "second"}}

	NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)

	body := rec.Body.String()
	assert.Equal(t, "data: \"first\"\n\nevent: error\ndata: {\"error\":\"stream value could not be encoded\"}\n\n", body)
	assert.NotContains(t, body, "second")
	assert.NotContains(t, body, "[DONE]")
}

// panicSource panics on Next, as a buggy provider might.
type panicSource struct{ closed bool }

func (*panicSource) Next() (any, bool) { panic("boom") }
func (*panicSource) Err() error        { return nil }
func (p *panicSource) Close() error    { p.closed = true; return nil }

// A panic in the source must be contained (it runs in a spawned goroutine that the handler's
// recovery middleware does not cover) and surfaced as an error frame, not crash the process.
func TestStream_SourcePanic_Contained(t *testing.T) {
	rec := httptest.NewRecorder()
	src := &panicSource{}

	require.NotPanics(t, func() {
		NewResponder(rec, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)
	})

	assert.Contains(t, rec.Body.String(), "event: error")
	assert.Contains(t, rec.Body.String(), "stream source panicked")
	assert.True(t, src.closed)
}

func TestStream_ImmediateWriteFailure(t *testing.T) {
	w := &erroringWriter{header: make(http.Header)}
	src := &sliceSource{items: []any{"a", "b"}}

	require.NotPanics(t, func() {
		NewResponder(w, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)
	})
	assert.True(t, src.closed, "source closed even when the first write fails")
}
