package gofr

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// Markers the shutdown helper process prints, and the env var that turns it from a skipped test
// into the app under test.
const (
	shutdownHelperEnv = "GOFR_RUN_SHUTDOWN_HELPER"
	shutdownWaited    = "MARKER:shutdown-finished-before-run-returned"
	shutdownRaced     = "MARKER:run-returned-first"
)

// closeRecordingLogger closes its channel when Shutdown closes the logger, which is the last thing
// App.Shutdown does. It tells the helper whether shutdown had finished at the moment Run returned,
// without racing the shutdown goroutine's own logging.
type closeRecordingLogger struct {
	logging.Logger

	closed chan struct{}
}

func (l *closeRecordingLogger) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}

	return nil
}

func TestApp_awaitShutdown(t *testing.T) {
	const timeout = 50 * time.Millisecond

	closedChan := func() chan struct{} {
		ch := make(chan struct{})
		close(ch)

		return ch
	}

	canceledCtx := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		return ctx
	}

	tests := []struct {
		desc        string
		ctx         context.Context
		done        chan struct{}
		maxDuration time.Duration
	}{
		{
			desc: "servers stopped without a termination signal, so nothing is waited for",
			ctx:  context.Background(),
			// Never closed: waiting on it would block until the test's own deadline.
			done:        make(chan struct{}),
			maxDuration: timeout,
		},
		{
			desc:        "shutdown already complete",
			ctx:         canceledCtx(),
			done:        closedChan(),
			maxDuration: timeout,
		},
		{
			desc:        "shutdown never completes, so the wait is bounded",
			ctx:         canceledCtx(),
			done:        make(chan struct{}),
			maxDuration: timeout + shutdownWaitMargin + time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			app := New()

			start := time.Now()

			app.awaitShutdown(tc.ctx, tc.done, timeout)

			assert.Less(t, time.Since(start), tc.maxDuration, "awaitShutdown blocked for too long")
		})
	}
}

// TestApp_Run_waitsForShutdown covers the signal path a typical main() takes: Run must not return
// — and so let the process exit — while the graceful shutdown it started is still running.
//
// The app runs in a helper process because SIGTERM is delivered to the whole process: sending it
// from this test would also tear down the apps that other tests in this package leave running.
func TestApp_Run_waitsForShutdown(t *testing.T) {
	httpPort := testutil.GetFreePort(t)

	// os.Executable, not os.Args[0]: other tests in this package overwrite os.Args to drive the
	// CMD app and do not always put it back.
	self, err := os.Executable()
	require.NoError(t, err)

	// Re-executing this test binary is the standard way to test signal handling.
	helper := exec.CommandContext(t.Context(), self, "-test.run=^TestShutdownHelperProcess$", "-test.v")
	helper.Env = append(os.Environ(),
		shutdownHelperEnv+"=1",
		"HTTP_PORT="+strconv.Itoa(httpPort),
		"METRICS_PORT="+strconv.Itoa(testutil.GetFreePort(t)),
	)
	// The helper must outlive the signal: Cancel would otherwise kill it outright.
	helper.Cancel = func() error { return helper.Process.Signal(syscall.SIGTERM) }

	out := &bytes.Buffer{}
	helper.Stdout = out
	helper.Stderr = out

	require.NoError(t, helper.Start())

	waited := make(chan error, 1)
	go func() { waited <- helper.Wait() }()

	require.Eventually(t, func() bool {
		//nolint:noctx // a readiness poll with its own client timeout.
		resp, err := (&http.Client{Timeout: time.Second}).Get("http://localhost:" + strconv.Itoa(httpPort) + "/hello")
		if err != nil {
			return false
		}

		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, 30*time.Second, 50*time.Millisecond, "helper app never became ready:\n%s", out)

	require.NoError(t, helper.Process.Signal(syscall.SIGTERM))

	select {
	case err := <-waited:
		require.NoError(t, err, "helper process failed:\n%s", out)
	case <-time.After(30 * time.Second):
		t.Fatalf("helper process did not exit after SIGTERM:\n%s", out)
	}

	assert.Contains(t, out.String(), shutdownWaited,
		"Run returned before the graceful shutdown completed:\n%s", out)
	assert.NotContains(t, out.String(), shutdownRaced)
}

// TestShutdownHelperProcess is the app under test for TestApp_Run_waitsForShutdown, run as its own
// process so the SIGTERM it is sent reaches nothing else. It skips unless the parent asked for it.
func TestShutdownHelperProcess(t *testing.T) {
	if os.Getenv(shutdownHelperEnv) != "1" {
		t.Skip("helper process for TestApp_Run_waitsForShutdown")
	}

	app := New()
	app.GET("/hello", func(*Context) (any, error) {
		return helloWorld, nil
	})

	shutdownComplete := make(chan struct{})
	app.container.Logger = &closeRecordingLogger{Logger: app.container.Logger, closed: shutdownComplete}

	app.Run()

	select {
	case <-shutdownComplete:
		t.Log(shutdownWaited)
	default:
		t.Log(shutdownRaced)
	}
}

// TestShutdownAfterFailedStartup_ReportsABadGracePeriod pins that a malformed SHUTDOWN_GRACE_PERIOD
// is reported on the abandoned-startup path.
//
// An earlier revision skipped the error here, reasoning that Run's normal path already logs it. That
// path is precisely the one an abandoned startup never reaches, so the misconfiguration was silent
// in the only situation this function runs in -- and the operator whose grace period is a typo finds
// out by watching a cleanup take the default instead of theirs, with nothing said.
//
// The default is still used: a bad grace period must not stop the cleanup, only be mentioned.
func TestShutdownAfterFailedStartup_ReportsABadGracePeriod(t *testing.T) {
	tests := []struct {
		desc      string
		period    string
		wantEntry bool
	}{
		{desc: "malformed period is reported", period: "not-a-duration", wantEntry: true},
		{desc: "valid period says nothing", period: "1s", wantEntry: false},
		{desc: "unset period says nothing", period: "", wantEntry: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logs := testutil.StderrOutputForFunc(func() {
				a := New()
				a.Config = config.NewMockConfig(map[string]string{"SHUTDOWN_GRACE_PERIOD": tc.period})

				a.shutdownAfterFailedStartup()
			})

			if tc.wantEntry {
				assert.Contains(t, logs, "invalid SHUTDOWN_GRACE_PERIOD",
					"a grace period that does not parse must be reported on this path")
			} else {
				assert.NotContains(t, logs, "invalid SHUTDOWN_GRACE_PERIOD",
					"a usable grace period must not be reported as invalid")
			}
		})
	}
}
