//go:build !race

// Allocation counts are only meaningful without the race detector, which adds
// bookkeeping allocations of its own -- with the original encoding/json engine
// this same measurement reads 8 normally and 11 under -race. The guard is
// therefore excluded from race builds rather than loosened to a tolerance that
// would no longer catch anything.

package middleware

import (
	"encoding/json"
	goversion "go/version"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/logging"
)

// Allocations of one request through the Logging middleware, per encoding/json
// engine; see TestLoggingChainAllocationsDoNotRegress.
const (
	// allocsOriginalEngine is the original engine, used up to Go 1.26 (and on later
	// Go with GOEXPERIMENT=nojsonv2).
	allocsOriginalEngine = 8
	// allocsJSONv2 is json/v2, the engine from Go 1.27. It is one more than the
	// original, all of it inside encoding/json: json/v2 copies values to the heap to
	// make them addressable (twice per entry, where the original engine allocated
	// once in time.Time.MarshalJSON). The middleware's own allocations are unchanged.
	allocsJSONv2 = 9
)

// pinnedRequestLogAllocs returns the exact allocation count for the encoding/json
// engine built in, or skips the test when that engine is the GOEXPERIMENT=jsonv2
// preview on Go 1.25 or 1.26. The preview writes like json/v2 but allocates 16 times
// per request against 9, and pinning a preview's number would make the guard track
// the experiment rather than this package's optimizations.
//
// The engine is detected at run time: the original one writes the U+FFFD that
// replaces an invalid UTF-8 byte escaped, json/v2 writes the raw character. A
// development Go version does not parse as go1.27 and is skipped as the preview.
func pinnedRequestLogAllocs(t *testing.T) float64 {
	t.Helper()

	b, err := json.Marshal(string([]byte{0xff}))
	if err != nil {
		t.Fatal(err)
	}

	if string(b) == `"\ufffd"` {
		return allocsOriginalEngine
	}

	if goversion.Compare(runtime.Version(), "go1.27") < 0 {
		t.Skipf("allocation count is not pinned for the GOEXPERIMENT=jsonv2 preview engine (%s)",
			runtime.Version())
	}

	return allocsJSONv2
}

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
// regression. Reverting either optimization adds one allocation.
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
	// such an upgrade, re-measure and update the constant in the same commit. The
	// count depends on which encoding/json engine is built in; see
	// pinnedRequestLogAllocs.
	wantAllocs := pinnedRequestLogAllocs(t)

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
		t.Errorf("request-log path allocates %.0f objects, expected exactly %.0f -- an "+
			"optimization in this package has regressed, or a toolchain change moved the "+
			"baseline", got, wantAllocs)
	}

	t.Logf("request-log path: %.0f allocations", got)
}
