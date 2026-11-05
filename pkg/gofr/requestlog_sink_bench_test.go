package gofr

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// BenchmarkRequestLogSink serves the defaults-mode plaintext route with the request log sent to
// different stdout sinks, serially and from parallel goroutines. "off" raises LOG_LEVEL past the
// request log, so off vs devnull is the whole cost of one request log line (encode + write
// syscall), devnull vs pipe is what a consumer on the other end of a pipe adds, and the parallel
// runs show the wait on the stdout file's write lock that concurrent requests share.
func BenchmarkRequestLogSink(b *testing.B) {
	sinks := []struct {
		name, logLevel string
		open           func(b *testing.B) *os.File
	}{
		{"off", "NOTICE", openDevNull},
		{"devnull", "", openDevNull},
		{"pipe", "", openDrainedPipe},
	}

	for _, s := range sinks {
		b.Run(s.name+"/serial", func(b *testing.B) {
			h := newScenarioAppTo(b, s.logLevel, 0, s.open(b))
			req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/plaintext", http.NoBody)
			w := &benchDiscardResponseWriter{}

			b.ReportAllocs()

			for b.Loop() {
				w.reset()
				h.ServeHTTP(w, req)
			}
		})

		b.Run(s.name+"/parallel", func(b *testing.B) {
			h := newScenarioAppTo(b, s.logLevel, 0, s.open(b))

			b.ReportAllocs()
			b.SetParallelism(32) // 64 goroutines at GOMAXPROCS=2, as the load test's -c 64

			b.RunParallel(func(pb *testing.PB) {
				req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/plaintext", http.NoBody)
				w := &benchDiscardResponseWriter{}

				for pb.Next() {
					w.reset()
					h.ServeHTTP(w, req)
				}
			})
		})
	}
}

func openDevNull(b *testing.B) *os.File {
	b.Helper()

	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() { f.Close() })

	return f
}

// openDrainedPipe returns the write end of a pipe that a goroutine drains as fast as it can.
func openDrainedPipe(b *testing.B) *os.File {
	b.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		b.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		_, _ = io.Copy(io.Discard, r)

		close(done)
	}()

	b.Cleanup(func() {
		w.Close()
		<-done
		r.Close()
	})

	return w
}
