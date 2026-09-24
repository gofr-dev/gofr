package gofr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/cmd/terminal"
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

// closeRecordingLogger closes its channel when the logger is closed, which is the last thing
// App.Shutdown does. It tells the helper whether shutdown had finished at the moment Run returned,
// without racing the shutdown goroutine's own logging. onClose, when set, is also called on every
// Close, so a test can record where the close falls relative to other steps.
type closeRecordingLogger struct {
	logging.Logger

	closed  chan struct{}
	onClose func()
}

func (l *closeRecordingLogger) Close() error {
	if l.onClose != nil {
		l.onClose()
	}

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

var errTraceFlush = errors.New("trace flush failed")

func TestApp_runCMD_FlushErrorIsLogged(t *testing.T) {
	tests := []struct {
		desc       string
		shutdown   func(context.Context) error
		setupMocks func(l *container.MockLogger)
	}{
		{
			desc:       "trace flush succeeds silently",
			shutdown:   func(context.Context) error { return nil },
			setupMocks: func(*container.MockLogger) {},
		},
		{
			desc:     "trace flush error is logged",
			shutdown: func(context.Context) error { return errTraceFlush },
			setupMocks: func(l *container.MockLogger) {
				l.EXPECT().Errorf("failed to flush traces: %v", errTraceFlush)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			oldArgs := os.Args

			t.Cleanup(func() { os.Args = oldArgs })

			os.Args = []string{"", "flush"}

			logger := container.NewMockLogger(gomock.NewController(t))
			tc.setupMocks(logger)

			var ran bool

			a := &App{
				cmd:            &cmd{out: terminal.New()},
				container:      &container.Container{Logger: logger},
				shutdownTracer: tc.shutdown,
			}
			a.SubCommand("flush", func(*Context) (any, error) {
				ran = true
				return nil, nil
			})

			a.Run()

			assert.True(t, ran)
		})
	}
}

func TestApp_Run_StartupHookCanceled(t *testing.T) {
	testutil.NewServerConfigs(t)

	var hookCalled bool

	out := testutil.StdoutOutputForFunc(func() {
		app := New()
		app.OnStart(func(*Context) error {
			hookCalled = true
			return context.Canceled
		})

		// Run must return on its own: no server is started after a canceled hook.
		app.Run()
	})

	assert.True(t, hookCalled)
	assert.Contains(t, out, "Startup canceled by context, shutting down gracefully.")
}

func TestApp_startMCPServer(t *testing.T) {
	tests := []struct {
		desc       string
		mcp        func(port int) *mcpServer
		setupMocks func(l *container.MockLogger, port int)
	}{
		{
			desc:       "no MCP server configured",
			mcp:        func(int) *mcpServer { return nil },
			setupMocks: func(*container.MockLogger, int) {},
		},
		{
			desc: "MCP server shut down before it started",
			mcp: func(port int) *mcpServer {
				return &mcpServer{port: port, handler: http.NotFoundHandler(), stopped: true}
			},
			setupMocks: func(l *container.MockLogger, port int) {
				l.EXPECT().Logf("Starting MCP server on port: %d", port)
				l.EXPECT().Logf("MCP server was shut down before it started on port: %d", port)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			port := testutil.GetFreePort(t)

			logger := container.NewMockLogger(gomock.NewController(t))
			tc.setupMocks(logger, port)

			a := &App{container: &container.Container{Logger: logger}, mcpServer: tc.mcp(port)}

			var wg sync.WaitGroup

			a.startMCPServer(&wg)
			wg.Wait()
		})
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

// TestApp_runCMD_cleanupOrder pins the order of runCMD's final steps: the telemetry flush (observed
// through the trace shutdown, which flushCMDTelemetry runs alongside the metrics flush), then the
// logger close, and only then — for a failed command — the exit. Exiting any earlier would skip
// the steps after it, dropping the final span batch or leaving the logger unclosed on exactly the
// runs where they matter most.
func TestApp_runCMD_cleanupOrder(t *testing.T) {
	testCases := []struct {
		desc       string
		handler    Handler
		wantEvents []string
	}{
		{
			desc:       "failing command",
			handler:    func(*Context) (any, error) { return nil, errTest },
			wantEvents: []string{"flush traces", "close logger", "exit(1)"},
		},
		{
			desc:       "succeeding command",
			handler:    func(*Context) (any, error) { return "ok", nil },
			wantEvents: []string{"flush traces", "close logger"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			original := os.Args

			t.Cleanup(func() { os.Args = original })

			os.Args = []string{"", "run"}

			var events []string

			record := func(event string) { events = append(events, event) }

			app := &App{
				cmd: &cmd{},
				container: &container.Container{Logger: &closeRecordingLogger{
					Logger:  logging.NewMockLogger(logging.ERROR),
					closed:  make(chan struct{}),
					onClose: func() { record("close logger") },
				}},
				shutdownTracer: func(context.Context) error {
					record("flush traces")

					return nil
				},
				exit: func(code int) { record(fmt.Sprintf("exit(%d)", code)) },
			}
			app.cmd.addRoute("run", tc.handler)

			testutil.StdoutOutputForFunc(func() {
				testutil.StderrOutputForFunc(app.runCMD)
			})

			assert.Equal(t, tc.wantEvents, events, "order of runCMD's final steps")
		})
	}
}
