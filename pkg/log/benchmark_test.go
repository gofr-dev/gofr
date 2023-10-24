package log

import (
	"io"
	"testing"
)

func BenchmarkStdOutLogging(t *testing.B) {
	l := NewLogger()

	for i := 0; i < t.N; i++ {
		l.Log("Hello")
	}
}

func BenchmarkDiscardLogging(t *testing.B) {
	l := NewMockLogger(io.Discard)

	for i := 0; i < t.N; i++ {
		l.Log("Hello")
	}
}
