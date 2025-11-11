package gofr

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRecoveryHandler_Recover(t *testing.T) {
	tests := []struct {
		name      string
		panicVal  any
		component string
	}{
		{
			name:      "string panic",
			panicVal:  "test panic",
			component: "test-component",
		},
		{
			name:      "error panic",
			panicVal:  errors.New("test error"),
			component: "error-component",
		},
		{
			name:      "integer panic",
			panicVal:  42,
			component: "int-component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := testutil.StderrOutputForFunc(func() {
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				handler := NewRecoveryHandler(mockLogger, tt.component)

				func() {
					defer handler.Recover()
					panic(tt.panicVal)
				}()
			})

			assert.Contains(t, logs, tt.component)
			assert.Contains(t, logs, "goroutine")
		})
	}
}

func TestRecoveryHandler_Recover_NoPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		handler := NewRecoveryHandler(mockLogger, "no-panic")

		func() {
			defer handler.Recover()
			// No panic
		}()
	})

	// Should not log anything if there's no panic
	assert.NotContains(t, logs, "no-panic")
}

func TestRecoveryHandler_RecoverWithCallback(t *testing.T) {
	callbackCalled := false
	var callbackErr error

	logs := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		handler := NewRecoveryHandler(mockLogger, "callback-test")

		func() {
			defer handler.RecoverWithCallback(func(err error) {
				callbackCalled = true
				callbackErr = err
			})
			panic("test panic with callback")
		}()
	})

	assert.True(t, callbackCalled, "callback should be called")
	assert.Error(t, callbackErr, "callback error should not be nil")
	assert.Contains(t, callbackErr.Error(), "callback-test")
	assert.Contains(t, logs, "callback-test")
}

func TestRecoveryHandler_RecoverWithCallback_NoCallback(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		handler := NewRecoveryHandler(mockLogger, "no-callback-test")

		// Should not panic even if callback is nil
		func() {
			defer handler.RecoverWithCallback(nil)
			panic("test panic without callback")
		}()
	})

	assert.Contains(t, logs, "no-callback-test")
}

func TestRecoveryHandler_RecoverWithChannel(t *testing.T) {
	panicChan := make(chan struct{})

	go func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		handler := NewRecoveryHandler(mockLogger, "channel-test")

		defer handler.RecoverWithChannel(panicChan)
		panic("test panic with channel")
	}()

	select {
	case <-panicChan:
		// Success - channel was closed
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for panic channel")
	}
}

func TestRecoveryHandler_RecoverWithChannel_NoChannel(t *testing.T) {
	done := make(chan struct{})

	go func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		handler := NewRecoveryHandler(mockLogger, "no-channel-test")

		defer handler.RecoverWithChannel(nil)
		defer close(done)
		panic("test panic without channel")
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for completion")
	}
}

func TestSafeGo(t *testing.T) {
	done := make(chan struct{})
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	SafeGo(mockLogger, "safe-go-test", func() {
		defer close(done)
		panic("test panic in SafeGo")
	})

	select {
	case <-done:
		// Success - goroutine completed
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SafeGo completion")
	}
}

func TestSafeGo_NoPanic(t *testing.T) {
	done := make(chan struct{})
	result := 0
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	SafeGo(mockLogger, "safe-go-no-panic", func() {
		result = 42
		close(done)
	})

	select {
	case <-done:
		assert.Equal(t, 42, result)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SafeGo completion")
	}
}

func TestSafeGoWithCallback(t *testing.T) {
	callbackCalled := false
	done := make(chan struct{})
	mockLogger := logging.NewMockLogger(logging.DEBUG)

	SafeGoWithCallback(mockLogger, "safe-go-callback-test", func() {
		panic("test panic in SafeGoWithCallback")
	}, func(err error) {
		callbackCalled = true
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "safe-go-callback-test")
		close(done)
	})

	select {
	case <-done:
		assert.True(t, callbackCalled)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestRecoveryLog_Structure(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		handler := NewRecoveryHandler(logger, "structure-test")

		func() {
			defer handler.Recover()
			panic("test panic for log structure")
		}()
	})

	assert.Contains(t, logs, "structure-test")
	assert.Contains(t, logs, "test panic for log structure")
	assert.Contains(t, logs, "goroutine") // Stack trace contains goroutine info
}

func TestRecoveryHandler_DifferentPanicTypes(t *testing.T) {
	testCases := []struct {
		name     string
		panicVal any
	}{
		{"string", "panic string"},
		{"error", errors.New("panic error")},
		{"int", 123},
		{"float", 45.67},
		{"bool", true},
		{"struct", struct{ msg string }{msg: "panic struct"}},
		{"nil pointer", (*int)(nil)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs := testutil.StderrOutputForFunc(func() {
				mockLogger := logging.NewMockLogger(logging.DEBUG)
				handler := NewRecoveryHandler(mockLogger, "type-test-"+tc.name)

				func() {
					defer handler.Recover()
					panic(tc.panicVal)
				}()
			})

			assert.Contains(t, logs, "type-test-"+tc.name)
		})
	}
}

func TestNewRecoveryHandler(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	component := "test-component"

	handler := NewRecoveryHandler(mockLogger, component)

	assert.NotNil(t, handler)
	assert.Equal(t, mockLogger, handler.logger)
	assert.Equal(t, component, handler.component)
}

func TestRecoveryHandler_ConcurrentPanics(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.DEBUG)
	count := 10
	done := make(chan struct{}, count)

	for i := 0; i < count; i++ {
		go func(id int) {
			handler := NewRecoveryHandler(mockLogger, "concurrent-test")
			func() {
				defer handler.Recover()
				panic("concurrent panic")
			}()
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < count; i++ {
		select {
		case <-done:
			// Success - goroutine completed
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for goroutine %d", i)
		}
	}
}
