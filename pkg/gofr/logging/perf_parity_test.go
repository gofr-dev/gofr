package logging

import (
	"bytes"
	"strings"
	"testing"
)

// TestLogEntryOutputMatchesLog is the parity guarantee behind the fast path.
//
// LogEntry exists only to avoid the []any a variadic Log allocates. If it ever
// produced different bytes, every service that logs requests would silently
// change its log format on upgrade -- a breaking change with no compile error to
// catch it.
func TestLogEntryOutputMatchesLog(t *testing.T) {
	msgs := []any{
		"a plain string",
		map[string]string{"k": "v"},
		struct {
			A int    `json:"a"`
			B string `json:"b"`
		}{1, "two"},
		42,
		nil,
	}

	for _, msg := range msgs {
		var viaLog, viaEntry bytes.Buffer

		newLoggerAt(INFO, &viaLog, &viaLog).Log(msg)
		newLoggerAt(INFO, &viaEntry, &viaEntry).LogEntry(msg)

		if stripTime(viaLog.String()) != stripTime(viaEntry.String()) {
			t.Errorf("output differs for %#v:\n Log:      %s\n LogEntry: %s",
				msg, viaLog.String(), viaEntry.String())
		}
	}
}

// TestErrorEntryOutputMatchesError pins the same parity on the error path, which
// is the one a 5xx takes.
func TestErrorEntryOutputMatchesError(t *testing.T) {
	var viaErr, viaEntry bytes.Buffer

	newLoggerAt(INFO, &viaErr, &viaErr).Error("boom")
	newLoggerAt(INFO, &viaEntry, &viaEntry).ErrorEntry("boom")

	if stripTime(viaErr.String()) != stripTime(viaEntry.String()) {
		t.Errorf("error output differs:\n Error:      %s\n ErrorEntry: %s",
			viaErr.String(), viaEntry.String())
	}
}

// TestLogEntryRespectsLevel pins that the fast path is gated exactly as Log is.
// A fast path that ignored the level would emit entries a configured service
// expects to be suppressed.
func TestLogEntryRespectsLevel(t *testing.T) {
	var buf bytes.Buffer

	l := newLoggerAt(ERROR, &buf, &buf)

	l.LogEntry("should be suppressed at ERROR")

	if buf.Len() != 0 {
		t.Errorf("LogEntry emitted below the configured level: %s", buf.String())
	}

	l.ErrorEntry("should survive at ERROR")

	if buf.Len() == 0 {
		t.Error("ErrorEntry was suppressed at ERROR")
	}
}

// TestLogEntryFollowsChangeLevel pins that a level raised at runtime -- which is
// what the remote logger does -- takes effect on the fast path too.
func TestLogEntryFollowsChangeLevel(t *testing.T) {
	var buf bytes.Buffer

	l := newLoggerAt(INFO, &buf, &buf)

	l.LogEntry("first")

	if buf.Len() == 0 {
		t.Fatal("entry suppressed at INFO")
	}

	buf.Reset()
	l.ChangeLevel(FATAL)
	l.LogEntry("second")

	if buf.Len() != 0 {
		t.Errorf("entry survived a level raise: %s", buf.String())
	}
}

// stripTime removes the timestamp field, which necessarily differs between two
// calls and is not what these tests are comparing.
func stripTime(s string) string {
	i := strings.Index(s, `"time":`)
	if i < 0 {
		return s
	}

	j := strings.Index(s[i:], `,`)
	if j < 0 {
		return s
	}

	return s[:i] + s[i+j+1:]
}
