package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/version"
)

// slowJSON is what encodeJSON wrote before the fast path: the reference every line must match.
func slowJSON(tb testing.TB, e *logEntry) string {
	tb.Helper()

	var buf bytes.Buffer

	require.NoError(tb, json.NewEncoder(&buf).Encode(e))

	return buf.String()
}

func fastJSON(e *logEntry) string {
	var buf bytes.Buffer

	(&logger{}).encodeJSON(e, &buf)

	return buf.String()
}

// TestEncodeJSONMatchesEncodingJSON pins the log format byte for byte: for every level, message
// shape, trace ID and time, the line encodeJSON writes is the line encoding/json writes. Entries
// that leave the fast path are in the table too, so the fallback is pinned as well.
func TestEncodeJSONMatchesEncodingJSON(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+30*60)
	times := []time.Time{
		time.Now(), time.Now().UTC(), time.Now().In(ist),
		time.Date(2026, 10, 30, 15, 4, 5, 0, time.UTC),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), // encoding/json fails this; both must write nothing
	}

	messages := []any{
		"request handled", "", "quote \" and <html> & \\", "multi\nline", "héllo",
		nil, 42, []any{"a", 1}, map[string]string{"k": "v"},
		struct {
			A string `json:"a"`
		}{"x"},
	}

	for _, level := range []Level{DEBUG, INFO, NOTICE, WARN, ERROR, FATAL, Level(0), Level(99)} {
		for _, tm := range times {
			for _, msg := range messages {
				for _, traceID := range []string{"", "0123456789abcdef0123456789abcdef"} {
					e := &logEntry{Level: level, Time: tm, Message: msg, TraceID: traceID, GofrVersion: version.Framework}

					var want bytes.Buffer

					_ = json.NewEncoder(&want).Encode(e)

					assert.Equal(t, want.String(), fastJSON(e), "entry %+v", e)
				}
			}
		}
	}
}

// TestEncodeJSONOneWritePerLine pins that a line reaches the writer in one Write, as Encode wrote
// it. Writers such as a file opened O_APPEND rely on that to keep concurrent lines whole.
func TestEncodeJSONOneWritePerLine(t *testing.T) {
	w := &countingWriter{}
	e := &logEntry{Level: INFO, Time: time.Now(), Message: "hello", GofrVersion: version.Framework}

	(&logger{}).encodeJSON(e, w)

	assert.Equal(t, 1, w.writes)
	assert.Equal(t, slowJSON(t, e), w.buf.String())
}

type countingWriter struct {
	buf    bytes.Buffer
	writes int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++

	return w.buf.Write(p)
}

// TestEncodeJSONPooledBufferDoesNotLeak runs many goroutines through one logger, each writing a
// different message to its own writer. If a pooled buffer were shared or reused too early, a line
// would carry another goroutine's bytes. Run with -race.
func TestEncodeJSONPooledBufferDoesNotLeak(t *testing.T) {
	l := &logger{}

	const goroutines, lines = 8, 200

	var wg sync.WaitGroup

	errs := make(chan string, goroutines)

	for g := range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := range lines {
				// Lengths vary so a reused buffer holds a longer earlier line.
				msg := fmt.Sprintf("g%d-i%d-%s", g, i, strings.Repeat("x", (g*31+i)%97))
				e := &logEntry{Level: INFO, Time: time.Now(), Message: msg, GofrVersion: version.Framework}

				var want, got bytes.Buffer

				_ = json.NewEncoder(&want).Encode(e)

				l.encodeJSON(e, &got)

				if want.String() != got.String() {
					errs <- fmt.Sprintf("want %q got %q", want.String(), got.String())

					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}

// TestEncodeJSONDropsLargeBuffers pins that a line longer than maxPooledLine is written correctly
// and its buffer is not kept for reuse.
func TestEncodeJSONDropsLargeBuffers(t *testing.T) {
	l := &logger{}
	e := &logEntry{Level: INFO, Time: time.Now(), Message: strings.Repeat("y", maxPooledLine+1), GofrVersion: version.Framework}

	var got bytes.Buffer

	l.encodeJSON(e, &got)

	assert.Equal(t, slowJSON(t, e), got.String())

	if bp, ok := l.lines.Get().(*[]byte); ok {
		assert.LessOrEqual(t, cap(*bp), maxPooledLine, "an oversized buffer must not return to the pool")
	}
}

// BenchmarkEncodeJSON measures one string-message log line, the fast path, against encoding/json.
func BenchmarkEncodeJSON(b *testing.B) {
	e := &logEntry{Level: INFO, Time: time.Now(), Message: "request handled successfully",
		TraceID: "0123456789abcdef0123456789abcdef", GofrVersion: version.Framework}

	b.Run("fast", func(b *testing.B) {
		l := &logger{}

		b.ReportAllocs()

		for range b.N {
			l.encodeJSON(e, discardWriter{})
		}
	})

	b.Run("encoding_json", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_ = json.NewEncoder(discardWriter{}).Encode(e)
		}
	})
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
