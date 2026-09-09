//go:build !race

// Allocation counts are only meaningful without the race detector, which adds
// bookkeeping allocations of its own -- this same measurement reads 8 normally
// and 11 under -race. The guard is therefore excluded from race builds rather
// than loosened to a tolerance that would no longer catch anything.

package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/logging"
)

// discardWriter is a ResponseWriter that allocates nothing, so the count below
// is the middleware's and not httptest's.
type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header       { return d.h }
func (*discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*discardWriter) WriteHeader(int)             {}

// TestLoggingChainAllocationsDoNotRegress guards the allocations removed from
// the request-log path.
//
// Both savings here -- taking the log entry through LogEntry instead of a
// variadic Log, and rendering the trace and span IDs into one buffer instead of
// two -- are invisible from the outside: the bytes logged are identical either
// way, so every other test in this file passes with them reverted. This counts
// instead.
//
// It pins the count exactly, because each saving is worth one allocation and a
// tolerance wide enough to absorb drift is also wide enough to hide a
// regression. Reverting either optimization takes this from 8 to 9.
//
// The span context is valid on purpose. GoFr installs an SDK provider with
// NeverSample when no exporter is configured, so a default deployment really
// does format both IDs on every request; measuring with an invalid one would
// skip the very work being guarded.
func TestLoggingChainAllocationsDoNotRegress(t *testing.T) {
	// Exact, not a ceiling with headroom. Each saving here is worth exactly one
	// allocation, so any slack at all makes the guard blind to losing one -- an
	// earlier version allowed 21 against a measured 18 and passed with BOTH
	// optimizations reverted. AllocsPerRun is deterministic for fixed code, so the
	// only thing that moves this number is a Go or dependency upgrade, and that is
	// worth a human looking at rather than absorbing silently. If this fails after
	// such an upgrade, re-measure and update the constant in the same commit.
	const wantAllocs = 8

	tid, err := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	if err != nil {
		t.Fatal(err)
	}

	sid, err := trace.SpanIDFromHex("b7ad6b7169203331")
	if err != nil {
		t.Fatal(err)
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid})

	// A real logger, not the mock. MockLogger does not implement LogEntry, so the
	// middleware would take the variadic fallback and the guard would be blind to
	// the very saving it exists to protect -- and it would measure the mock's own
	// fmt.Fprintf to stdout rather than the framework's work. NewFileLogger with an
	// empty path writes to io.Discard.
	handler := Logging(LogProbes{}, logging.NewFileLogger(""))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "ok")
		}))

	req := httptest.NewRequestWithContext(
		trace.ContextWithSpanContext(t.Context(), sc), http.MethodGet, "/x", http.NoBody)
	w := &discardWriter{h: make(http.Header, 4)}

	// Warm the pooled status writer so the measured runs see the steady state.
	for range 5 {
		handler.ServeHTTP(w, req)
	}

	got := testing.AllocsPerRun(300, func() { handler.ServeHTTP(w, req) })

	if got != wantAllocs {
		t.Errorf("request-log path allocates %.0f objects, expected exactly %d -- an "+
			"optimization in this package has regressed, or a toolchain change moved the "+
			"baseline", got, wantAllocs)
	}

	t.Logf("request-log path: %.0f allocations", got)
}
