package http

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

// drainToRecorder streams src to completion through a recorder (a cooperative client).
func drainToRecorder(src resTypes.Streamer) {
	NewResponder(httptest.NewRecorder(), http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)
}

// disconnectMidStream models a client that drops while the source is blocked on the network.
func disconnectMidStream() {
	src := newBlockingSource()
	done := make(chan struct{})

	go func() {
		defer close(done)

		NewResponder(&erroringWriter{header: make(http.Header)}, http.MethodGet).
			Respond(resTypes.Stream{Source: src, Heartbeat: 5 * time.Millisecond}, nil)
	}()

	<-done
}

// Every stream lifecycle — clean drain, mid-stream disconnect, source error, and source panic —
// must release the reader goroutine. After many mixed cycles the goroutine count returns to
// baseline; a leak on any path would make it grow.
func TestStream_NoGoroutineLeak_ManyCycles(t *testing.T) {
	baseline := runtime.NumGoroutine()

	for range 50 {
		drainToRecorder(&sliceSource{items: []any{"a", "b", "c"}})           // clean completion
		disconnectMidStream()                                                // client disconnect
		drainToRecorder(&sliceSource{items: []any{"x"}, err: errClientGone}) // source error
		drainToRecorder(&panicSource{})                                      // source panic
	}

	runtime.GC()
	assertGoroutinesReturn(t, baseline)
}

// The source must be closed on every exit path, so its resources (connection, buffers) are always
// released — the other half of "no leak" alongside the goroutine check.
func TestStream_SourceClosedOnEveryExitPath(t *testing.T) {
	t.Run("clean completion", func(t *testing.T) {
		src := &sliceSource{items: []any{"a"}}
		drainToRecorder(src)
		assert.True(t, src.closed)
	})

	t.Run("source error", func(t *testing.T) {
		src := &sliceSource{items: []any{"a"}, err: errClientGone}
		drainToRecorder(src)
		assert.True(t, src.closed)
	})

	t.Run("source panic", func(t *testing.T) {
		src := &panicSource{}
		drainToRecorder(src)
		assert.True(t, src.closed)
	})

	t.Run("client disconnect", func(t *testing.T) {
		src := &sliceSource{items: []any{"a", "b"}}
		NewResponder(&erroringWriter{header: make(http.Header)}, http.MethodGet).
			Respond(resTypes.Stream{Source: src}, nil)
		assert.True(t, src.closed)
	})
}

// countingSource produces values without end, recording how many it has produced.
type countingSource struct {
	produced atomic.Int64
	closed   atomic.Bool
}

func (c *countingSource) Next() (any, bool) {
	c.produced.Add(1)
	return "x", true
}

func (*countingSource) Err() error { return nil }

func (c *countingSource) Close() error {
	c.closed.Store(true)
	return nil
}

// gatedWriter blocks on its first write until released, then reports the client gone.
type gatedWriter struct {
	header  http.Header
	release chan struct{}
	once    atomic.Bool
}

func (w *gatedWriter) Header() http.Header { return w.header }

func (w *gatedWriter) Write([]byte) (int, error) {
	if w.once.CompareAndSwap(false, true) {
		<-w.release
	}

	return 0, errClientGone
}

func (*gatedWriter) WriteHeader(int) {}
func (*gatedWriter) Flush()          {}

// An unbounded source must not run ahead of a stalled client: the unbuffered handoff means at most a
// couple of values are in flight, so memory stays bounded instead of buffering the whole stream.
func TestStream_Backpressure_BoundedProduction(t *testing.T) {
	src := &countingSource{}
	w := &gatedWriter{header: make(http.Header), release: make(chan struct{})}

	done := make(chan struct{})

	go func() {
		defer close(done)

		NewResponder(w, http.MethodGet).Respond(resTypes.Stream{Source: src}, nil)
	}()

	// While the client is stalled in Write, the producer can be at most a value or two ahead of the
	// unbuffered channel — never the thousands an unbounded buffer would allow.
	time.Sleep(100 * time.Millisecond)
	assert.LessOrEqual(t, src.produced.Load(), int64(3),
		"producer must block on backpressure, not buffer ahead")

	close(w.release) // client "gone" — the write fails and the stream tears down
	<-done

	assert.True(t, src.closed.Load(), "source is closed once the stalled client drops")
}
