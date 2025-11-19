package gofr

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

// TestIntegration_ServerKeepsRunningAfterPanic verifies that the server remains operational
// after a handler panic is recovered.
func TestIntegration_ServerKeepsRunningAfterPanic(t *testing.T) {
	ports := testutil.NewServerConfigs(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(ports.MetricsPort))
	t.Setenv("HTTP_PORT", strconv.Itoa(ports.HTTPPort))

	var panicHandlerCalls atomic.Int64
	var normalHandlerCalls atomic.Int64

	app := New()

	// Handler that panics
	app.GET("/panic", func(c *Context) (any, error) {
		panicHandlerCalls.Add(1)
		panic("intentional panic for testing")
	})

	// Normal handler to verify server is still operational
	app.GET("/health", func(c *Context) (any, error) {
		normalHandlerCalls.Add(1)
		return map[string]string{"status": "ok"}, nil
	})

	// Start the server in a goroutine
	go func() {
		app.Run()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	// Make request to panic endpoint
	panicResp, err := client.Get(fmt.Sprintf("http://localhost:%d/panic", ports.HTTPPort))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, panicResp.StatusCode)
	panicResp.Body.Close()

	// Verify panic handler was called
	assert.Equal(t, int64(1), panicHandlerCalls.Load())

	// Make multiple requests to health endpoint to verify server is still operational
	for i := 0; i < 3; i++ {
		healthResp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", ports.HTTPPort))
		require.NoError(t, err, "request %d failed", i)
		assert.Equal(t, http.StatusOK, healthResp.StatusCode, "request %d returned wrong status", i)
		healthResp.Body.Close()
	}

	// Verify health handler was called 3 times
	assert.Equal(t, int64(3), normalHandlerCalls.Load())
}

// TestIntegration_MultipleConsecutivePanics verifies that the server can recover from multiple panics.
func TestIntegration_MultipleConsecutivePanics(t *testing.T) {
	ports := testutil.NewServerConfigs(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(ports.MetricsPort))
	t.Setenv("HTTP_PORT", strconv.Itoa(ports.HTTPPort))

	var panicCount atomic.Int64

	app := New()

	app.GET("/panic", func(c *Context) (any, error) {
		panicCount.Add(1)
		panic(fmt.Sprintf("panic number %d", panicCount.Load()))
	})

	app.GET("/status", func(c *Context) (any, error) {
		return map[string]int64{"panic_count": panicCount.Load()}, nil
	})

	go func() {
		app.Run()
	}()

	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	// Make multiple panic requests
	for i := 0; i < 5; i++ {
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/panic", ports.HTTPPort))
		require.NoError(t, err, "panic request %d failed", i)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "panic request %d returned wrong status", i)
		resp.Body.Close()
	}

	// Verify all panics were recovered
	assert.Equal(t, int64(5), panicCount.Load())

	// Verify server is still operational
	statusResp, err := client.Get(fmt.Sprintf("http://localhost:%d/status", ports.HTTPPort))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusResp.StatusCode)
	statusResp.Body.Close()
}
