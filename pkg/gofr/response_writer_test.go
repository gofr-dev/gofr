package gofr

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestResponseWriter_Error(t *testing.T) {
	w := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
	}

	testErr := errors.New("test error")
	w.Error(testErr)

	if got := w.GetError(); got != testErr {
		t.Errorf("responseWriter.GetError() = %v, want %v", got, testErr)
	}
} 
