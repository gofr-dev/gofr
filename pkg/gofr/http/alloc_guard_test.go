//go:build !race

// Allocation counts are only meaningful without the race detector, which adds
// bookkeeping allocations of its own. The guard is excluded from race builds
// rather than loosened to a tolerance that would no longer catch anything.

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestTrieRequestAllocationsDoNotRegress is the guard the optimizations in this
// package lacked.
//
// Every test beside it checks behavior, which all three changes preserve by
// design -- a memoized chain, a skipped empty map and a returned slice are all
// invisible from the outside. So a revert of any of them keeps the suite green
// and gives the allocations back silently. This counts them instead.
//
// The ceiling is a ceiling, not the measurement: it sits above the figure the
// commit quotes so ordinary variation in the runtime or a dependency does not
// fail the build, while a regression of the size these changes made does.
func TestTrieRequestAllocationsDoNotRegress(t *testing.T) {
	t.Setenv(RouterEnvVar, MatcherTrie)

	// Exact, not a ceiling. With headroom this caught only the largest of the three
	// savings here: reverting the chain memoization costs five allocations and was
	// caught, but the empty-Vars skip costs two and the collect signature one, and
	// both slipped under a tolerance wide enough to absorb toolchain drift.
	// AllocsPerRun is deterministic for fixed code, so the only thing that moves
	// this is a Go or dependency upgrade -- worth a human looking at. If it fails
	// after one, re-measure and update the constant in the same commit.
	const wantAllocs = 7

	r := NewRouter()
	// Five middlewares, because that is what newHTTPServer installs and because the
	// saving is per middleware: composing the chain per request allocates one
	// closure for each. A single middleware would make the difference one
	// allocation, small enough to hide under the ceiling's headroom, and the guard
	// would pass with the optimization reverted.
	for range 5 {
		r.UseMiddleware(func(inner http.Handler) http.Handler {
			// Returns a NEW handler, as every real middleware does. One that handed
			// back `inner` unchanged would allocate nothing when composed, and the
			// guard would then pass with the memoization reverted.
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				inner.ServeHTTP(w, req)
			})
		})
	}

	r.Add(http.MethodGet, "/bench/ping", http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))

	// The request and writer are built once and reused, as the benchmark beside
	// this does. Allocating them inside the measured closure would count
	// httptest's own work -- about 17 objects -- and drown the thing being
	// guarded. ServeHTTP does not mutate the request it is given; the router
	// copies it when it attaches context.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/bench/ping", http.NoBody)
	w := &nopWriter{header: make(http.Header, 4)}

	// Warm the lazily built trie index and the per-route chain, so the measured
	// runs see the steady state rather than one-time construction.
	for range 5 {
		r.ServeHTTP(w, req)
	}

	got := testing.AllocsPerRun(200, func() {
		r.ServeHTTP(w, req)
	})

	if got != wantAllocs {
		t.Errorf("trie request path allocates %.0f objects, expected exactly %d -- an "+
			"optimization in this package has regressed, or a toolchain change moved the "+
			"baseline", got, wantAllocs)
	}

	t.Logf("trie request path: %.0f allocations", got)
}
