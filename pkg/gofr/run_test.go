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

	"gofr.dev/pkg/gofr/container"
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

// TestApp_runCMD_exitCode drives runCMD end to end and records what it reports to the process
// through the exit seam: a failed command must exit with exitCodeCommandFailed, and anything else
// must not exit at all (returning from main is exit status 0). The expected code is the literal 1,
// not the constant, because it is the contract shells and CI branch on.
func TestApp_runCMD_exitCode(t *testing.T) {
	testCases := []struct {
		desc       string
		args       []string
		handler    Handler
		wantExits  []int
		wantStdout string
		wantStderr string
	}{
		{
			desc:       "handler returns nil error",
			args:       []string{"", "run"},
			handler:    func(*Context) (any, error) { return "ok", nil },
			wantExits:  nil,
			wantStdout: "ok\n",
		},
		{
			desc:       "handler returns error",
			args:       []string{"", "run"},
			handler:    func(*Context) (any, error) { return nil, errTest },
			wantExits:  []int{1},
			wantStderr: errTest.Error() + "\n",
		},
		{
			desc:       "handler returns data and error",
			args:       []string{"", "run"},
			handler:    func(*Context) (any, error) { return "partial", errTest },
			wantExits:  []int{1},
			wantStdout: "partial\n",
			wantStderr: errTest.Error() + "\n",
		},
		{
			desc:       "unknown command",
			args:       []string{"", "does-not-exist"},
			handler:    func(*Context) (any, error) { return "ok", nil },
			wantExits:  []int{1},
			wantStdout: "Available commands:",
			wantStderr: "'does-not-exist' is not a valid command.\n",
		},
		{
			desc:       "missing command",
			args:       []string{""},
			handler:    func(*Context) (any, error) { return "ok", nil },
			wantExits:  []int{1},
			wantStdout: "Available commands:",
			wantStderr: "'' is not a valid command.\n",
		},
		{
			desc:       "help flag",
			args:       []string{"", "--help"},
			handler:    func(*Context) (any, error) { return "ok", nil },
			wantExits:  nil,
			wantStdout: "Available commands:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			original := os.Args

			t.Cleanup(func() { os.Args = original })

			os.Args = tc.args

			var exits []int

			app := &App{
				cmd:       &cmd{},
				container: &container.Container{Logger: logging.NewMockLogger(logging.ERROR)},
				exit:      func(code int) { exits = append(exits, code) },
			}
			app.cmd.addRoute("run", tc.handler)

			var stderr string

			stdout := testutil.StdoutOutputForFunc(func() {
				stderr = testutil.StderrOutputForFunc(app.runCMD)
			})

			assert.Equal(t, tc.wantExits, exits, "exit codes reported to the process")
			assert.Contains(t, stdout, tc.wantStdout, "stdout")
			assert.Equal(t, tc.wantStderr, stderr, "stderr")
		})
	}
}
