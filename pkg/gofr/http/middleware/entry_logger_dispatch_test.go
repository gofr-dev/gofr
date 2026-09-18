package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	assert.Nil(t, newLogSink(nil).logger, "a nil logger must not be wrapped into a non-nil sink")
}
