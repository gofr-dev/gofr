package gofr

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

func TestShutdownWithContext_ContextTimeout(t *testing.T) {
	// Mock shutdown function that never completes
	mockShutdownFunc := func(ctx context.Context) error {
		// Simulate a long-running process
		<-ctx.Done()
		return nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := ShutdownWithContext(ctx, mockShutdownFunc, nil)

	require.ErrorIs(t, err, context.DeadlineExceeded, "Expected context deadline exceeded error")
}

func TestShutdownWithContext_SuccessfulShutdown(t *testing.T) {
	// Mock shutdown function that completes successfully
	mockShutdownFunc := func(_ context.Context) error {
		// Simulate a quick shutdown
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	err := ShutdownWithContext(ctx, mockShutdownFunc, nil)

	require.NoError(t, err, "Expected successful shutdown without error")
}

func Test_getShutdownTimeoutFromConfig_Success(t *testing.T) {
	tests := []struct {
		name          string
		configValue   string
		expectedValue time.Duration
	}{
		{"Valid timeout", "1s", 1 * time.Second},
		{"Empty timeout", "", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(map[string]string{
				"SHUTDOWN_GRACE_PERIOD": tt.configValue,
			})

			timeout, err := getShutdownTimeoutFromConfig(mockConfig)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedValue, timeout)
		})
	}
}

func Test_getShutdownTimeoutFromConfig_Error(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"SHUTDOWN_GRACE_PERIOD": "invalid",
	})

	_, err := getShutdownTimeoutFromConfig(mockConfig)
	require.Error(t, err)
}

// freeTCPPort returns a port that was free at the moment it was probed. The
// servers under test take a port number rather than a listener, so there is no
// way to ask them to bind :0 and report back.
func freeTCPPort(t *testing.T) int {
	t.Helper()

	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := l.Addr().(*net.TCPAddr).Port

	require.NoError(t, l.Close())

	return port
}

// isListening reports whether anything accepts a connection on the port.
func isListening(ctx context.Context, port int) bool {
	dialer := net.Dialer{Timeout: 200 * time.Millisecond}

	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}

	conn.Close()

	return true
}

// A termination signal can arrive before the serve goroutine has published its
// *http.Server — Run starts the shutdown handler before the servers. Shutdown
// must record that stop was requested, so the start that follows is abandoned
// instead of serving a listener nobody will ever stop.
func TestShutdown_BeforeStart_AbandonsTheStart(t *testing.T) {
	tests := []struct {
		name   string
		server func(port int) (run func(*container.Container), shutdown func(context.Context) error)
	}{
		{"http server", func(port int) (func(*container.Container), func(context.Context) error) {
			s := &httpServer{router: gofrHTTP.NewRouter(), port: port}
			return s.run, s.Shutdown
		}},
		{"metrics server", func(port int) (func(*container.Container), func(context.Context) error) {
			m := newMetricServer(port)
			return m.Run, m.Shutdown
		}},
		{"mcp server", func(port int) (func(*container.Container), func(context.Context) error) {
			m := newMCPServer(port, http.NotFoundHandler())
			return m.Run, m.Shutdown
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := freeTCPPort(t)
			run, shutdown := tt.server(port)
			c := &container.Container{Logger: logging.NewLogger(logging.ERROR)}

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			require.NoError(t, shutdown(ctx), "shutting down a server that has not started is not an error")

			returned := make(chan struct{})

			go func() {
				defer close(returned)

				run(c)
			}()

			select {
			case <-returned:
			case <-time.After(2 * time.Second):
				t.Fatal("run() did not return after Shutdown: the server started anyway and nothing will stop it")
			}

			assert.False(t, isListening(t.Context(), port), "port %d is being served after Shutdown", port)
		})
	}
}

// The signal window is a race, not a fixed ordering: Shutdown can land before
// run() publishes its server, after it, or while it is publishing. Whichever way
// the two goroutines interleave, the process must not be left serving.
func TestShutdown_ConcurrentWithStart_LeavesNothingServing(t *testing.T) {
	tests := []struct {
		name   string
		server func(port int) (run func(*container.Container), shutdown func(context.Context) error)
	}{
		{"http server", func(port int) (func(*container.Container), func(context.Context) error) {
			s := &httpServer{router: gofrHTTP.NewRouter(), port: port}
			return s.run, s.Shutdown
		}},
		{"metrics server", func(port int) (func(*container.Container), func(context.Context) error) {
			m := newMetricServer(port)
			return m.Run, m.Shutdown
		}},
		{"mcp server", func(port int) (func(*container.Container), func(context.Context) error) {
			m := newMCPServer(port, http.NotFoundHandler())
			return m.Run, m.Shutdown
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := freeTCPPort(t)
			run, shutdown := tt.server(port)
			c := &container.Container{Logger: logging.NewLogger(logging.ERROR)}

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			var wg sync.WaitGroup

			wg.Add(2)

			go func() {
				defer wg.Done()

				run(c)
			}()

			go func() {
				defer wg.Done()

				assert.NoError(t, shutdown(ctx))
			}()

			done := make(chan struct{})

			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("run() outlived Shutdown: the server is serving with nothing left to stop it")
			}

			assert.False(t, isListening(t.Context(), port), "port %d is being served after Shutdown", port)
		})
	}
}
