package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// countingEntryLogger implements both the plain logger contract and the
// entryLogger fast path, and records which one the middleware actually used.
type countingEntryLogger struct {
	logCalls, errorCalls           int
	logEntryCalls, errorEntryCalls int
}

func (c *countingEntryLogger) Log(...any)     { c.logCalls++ }
func (c *countingEntryLogger) Error(...any)   { c.errorCalls++ }
func (c *countingEntryLogger) LogEntry(any)   { c.logEntryCalls++ }
func (c *countingEntryLogger) ErrorEntry(any) { c.errorEntryCalls++ }

// TestEntryLoggerFastPathIsTaken pins that the middleware really uses the
// optional interface when the logger offers it.
//
// This is the one thing the rest of the suite cannot catch. LogEntry and Log
// produce byte-identical output by design, so a type assertion that silently
// stopped matching -- a renamed method, a pointer/value receiver mismatch, an
// interface that drifted -- would keep every output test passing while quietly
// giving back the allocation the fast path exists to save. Only counting which
// method was called can tell the difference.
func TestEntryLoggerFastPathIsTaken(t *testing.T) {
	logger := &countingEntryLogger{}

	handler := Logging(LogProbes{}, logger)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody))

	assert.Equal(t, 1, logger.logEntryCalls, "the middleware must use the LogEntry fast path")
	assert.Zero(t, logger.logCalls, "the variadic Log path allocates the slice this avoids")
}

// TestErrorEntryFastPathIsTaken is the same guarantee for the 5xx path, which
// takes a different branch and could regress independently.
func TestErrorEntryFastPathIsTaken(t *testing.T) {
	logger := &countingEntryLogger{}

	handler := Logging(LogProbes{}, logger)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody))

	assert.Equal(t, 1, logger.errorEntryCalls, "a 5xx must use the ErrorEntry fast path")
	assert.Zero(t, logger.errorCalls)
}

// plainOnlyLogger offers only the base contract, so the middleware must fall
// back -- the compatibility half of the same guarantee.
type plainOnlyLogger struct{ logCalls, errorCalls int }

func (p *plainOnlyLogger) Log(...any)   { p.logCalls++ }
func (p *plainOnlyLogger) Error(...any) { p.errorCalls++ }

// TestFallbackForLoggerWithoutFastPath pins that an external logger that never
// heard of LogEntry keeps working unchanged.
func TestFallbackForLoggerWithoutFastPath(t *testing.T) {
	logger := &plainOnlyLogger{}

	handler := Logging(LogProbes{}, logger)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody))

	assert.Equal(t, 1, logger.logCalls, "a logger without the fast path must still be logged through")
}

// gatedEntryLogger offers both optional interfaces, so newLogSink must resolve
// both of them.
type gatedEntryLogger struct{ countingEntryLogger }

func (*gatedEntryLogger) LogEnabled() bool { return true }

// TestLogSinkResolvesOptionalInterfacesOnce pins the hoist: the assertions
// happen when the middleware is built, not per request.
//
// The dispatch tests above prove the fast path is reached; they cannot tell
// where it was selected, so a move back into the request path would keep them
// green. Asserting on the resolved sink is what fixes the construction-time
// contract, which is the one newHistogramRecorder and remotelogger.New already
// follow.
func TestLogSinkResolvesOptionalInterfacesOnce(t *testing.T) {
	both := newLogSink(&gatedEntryLogger{})
	assert.NotNil(t, both.entries, "a logger with LogEntry/ErrorEntry must resolve the entry fast path")
	assert.NotNil(t, both.enabler, "a logger with LogEnabled must resolve the level gate")

	plain := newLogSink(&plainOnlyLogger{})
	assert.Nil(t, plain.entries, "a plain logger must leave the fast path unresolved")
	assert.Nil(t, plain.enabler, "a plain logger must leave the level gate unresolved")
	assert.NotNil(t, plain.logger, "the base contract is always kept")

	// The two optional interfaces are independent, and a logger in the wild can
	// offer either one alone -- remotelogger gained LogEnabled before it gained
	// LogEntry, so both partial shapes have actually existed in this repo. The
	// all-or-nothing cases above would pass even if the two assertions were
	// accidentally collapsed into one; these are what separate them.
	entriesOnly := newLogSink(&countingEntryLogger{})
	assert.NotNil(t, entriesOnly.entries, "LogEntry/ErrorEntry alone must still resolve the fast path")
	assert.Nil(t, entriesOnly.enabler, "a logger without LogEnabled must leave the level gate unresolved")

	enablerOnly := newLogSink(&levelGateLogger{})
	assert.Nil(t, enablerOnly.entries, "a logger without LogEntry must leave the fast path unresolved")
	assert.NotNil(t, enablerOnly.enabler, "LogEnabled alone must still resolve the level gate")

	assert.Nil(t, newLogSink(nil).logger, "a nil logger must not be wrapped into a non-nil sink")
}

// concurrentEntryLogger counts through atomics so the counters themselves cannot
// be what a race detector reports.
type concurrentEntryLogger struct {
	logCalls, errorCalls           atomic.Int64
	logEntryCalls, errorEntryCalls atomic.Int64
}

func (c *concurrentEntryLogger) Log(...any)     { c.logCalls.Add(1) }
func (c *concurrentEntryLogger) Error(...any)   { c.errorCalls.Add(1) }
func (c *concurrentEntryLogger) LogEntry(any)   { c.logEntryCalls.Add(1) }
func (c *concurrentEntryLogger) ErrorEntry(any) { c.errorEntryCalls.Add(1) }
func (*concurrentEntryLogger) LogEnabled() bool { return true }

// TestSharedLogSinkIsSafeUnderConcurrentRequests exercises the hoisted sink from
// many requests at once, which is the whole point of resolving it once: one
// logSink value is now shared by every request the middleware ever serves.
//
// Test_LoggingContract_ConcurrentRequestsShareThePool already runs concurrent
// requests under -race, but its recorder implements only Log/Error, so it
// exercises the FALLBACK branch and would stay green if the shared fast-path
// fields were unsafe. This drives the entries and enabler branches instead.
func TestSharedLogSinkIsSafeUnderConcurrentRequests(t *testing.T) {
	logger := &concurrentEntryLogger{}
	mw := Logging(LogProbes{}, logger)

	const (
		n     = 64
		half  = n / 2
		route = "/concurrent"
	)

	var wg sync.WaitGroup

	for i := range n {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			status := http.StatusOK
			if i%2 == 0 {
				status = http.StatusInternalServerError
			}

			h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))

			h.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequestWithContext(context.Background(), http.MethodGet, route, http.NoBody))
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int64(half), logger.logEntryCalls.Load(), "every 2xx must reach the shared fast path")
	assert.Equal(t, int64(half), logger.errorEntryCalls.Load(), "every 5xx must reach the shared fast path")
	assert.Zero(t, logger.logCalls.Load(), "no request may fall back to the variadic path")
	assert.Zero(t, logger.errorCalls.Load(), "no request may fall back to the variadic path")
}
