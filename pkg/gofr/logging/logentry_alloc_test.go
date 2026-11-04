//go:build !race

package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// LogEntry with a message the fast encoder handles allocates nothing: the entry it builds stays
// on the stack, and the line buffer is pooled. The race detector adds allocations of its own, so
// this only builds without it.
func TestLogEntry_DoesNotAllocate(t *testing.T) {
	l, ok := NewFileLogger("").(*logger)
	if !ok {
		t.Fatal("NewFileLogger did not return *logger")
	}

	msg := any("request served")

	l.LogEntry(msg)

	assert.Zero(t, testing.AllocsPerRun(200, func() { l.LogEntry(msg) }))
}
