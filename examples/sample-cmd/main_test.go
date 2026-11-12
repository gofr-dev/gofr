package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd"
	"gofr.dev/pkg/gofr/cmd/terminal"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// subprocessEnv turns the test binary into the sample CMD app: TestMain then runs main() with the
// arguments after "--" instead of running the tests. See runMainInSubprocess.
const subprocessEnv = "GOFR_SAMPLE_CMD_SUBPROCESS"

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	if os.Getenv(subprocessEnv) == "1" {
		sep := slices.Index(os.Args, "--")
		if sep < 0 {
			// The variable leaked into a normal test run; fail loudly rather than run main()
			// with the test binary's own flags and skip every test.
			fmt.Fprintf(os.Stderr, "%s is set but no -- separator was passed\n", subprocessEnv)
			os.Exit(2)
		}

		os.Args = append([]string{"command"}, os.Args[sep+1:]...)

		main()

		return
	}

	m.Run()
}

// runMainInSubprocess runs main() with args in a child copy of the test binary and returns its
// stdout, stderr and exit code: 0 when the child exits cleanly, otherwise the status it exited
// with. It asserts nothing about that code, so callers check it; any failure other than a non-zero
// exit (the child could not be started, for instance) fails the test.
//
// A failing command exits the process with a non-zero status, so it cannot run in-process
// without taking the test binary down with it. -test.run pins the child to the calling test, so
// that even if subprocessEnv were ignored the child would not re-run the whole suite.
func runMainInSubprocess(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	c := exec.CommandContext(t.Context(), os.Args[0], append([]string{"-test.run=^" + t.Name() + "$", "--"}, args...)...)
	c.Env = append(os.Environ(), subprocessEnv+"=1", "GOFR_TELEMETRY=false")

	var outBuf, errBuf bytes.Buffer

	c.Stdout, c.Stderr = &outBuf, &errBuf

	err := c.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return outBuf.String(), errBuf.String(), exitErr.ExitCode()
	}

	require.NoError(t, err, "running the child process")

	return outBuf.String(), errBuf.String(), 0
}

// setArgs replaces os.Args for the duration of the test and restores it
// afterwards.
//
// The CMD app reads os.Args directly, so a test that does not set it inherits
// the test binary's own flags — under CI the coverage run appends
// -test.testlogfile=..., which the CMD router then reports as an unknown
// subcommand. Tests must therefore own os.Args rather than whatever the
// harness was invoked with.
func setArgs(t *testing.T, args ...string) {
	t.Helper()

	original := os.Args
	t.Cleanup(func() { os.Args = original })

	os.Args = args
}

// TestCMDRunWithNoArg checks that if no subcommand is given then the
// framework's "not a valid command" error (with an empty command name) is
// written to stderr and the process exits with status 1.
func TestCMDRunWithNoArg(t *testing.T) {
	stdout, stderr, exitCode := runMainInSubprocess(t)

	assert.Equal(t, 1, exitCode, "exit code")
	assert.Equal(t, "'' is not a valid command.\n", stderr, "stderr")
	assert.Contains(t, stdout, "Available commands:", "stdout")
}

func TestCMDRunWithProperArg(t *testing.T) {
	expResp := "Hello World!\n"
	setArgs(t, "command", "hello")

	output := testutil.StdoutOutputForFunc(main)

	assert.Contains(t, output, expResp, "TEST Failed.\n")
}

func TestCMDRunWithParams(t *testing.T) {
	expResp := "Hello Vikash!\n"

	commands := []string{
		"command params -name=Vikash",
		"command params   -name=Vikash",
		"command -name=Vikash params",
		"command params -name=Vikash -",
	}

	for i, command := range commands {
		setArgs(t, strings.Split(command, " ")...)
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, expResp, "TEST[%d], Failed.\n", i)
	}
}

func TestCMDRun_Spinner(t *testing.T) {
	setArgs(t, "command", "spinner")
	output := testutil.StdoutOutputForFunc(main)

	// contains the spinner in the correct order
	assert.Contains(t, output, "\r⣾ \r⣽ \r⣻ \r⢿ \r⡿")
	// contains the process completion message
	assert.Contains(t, output, "Process Complete\n")
}

func TestCMDRun_SpinnerContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// add an already canceled context
	res, err := spinner(&gofr.Context{
		Context:   ctx,
		Request:   cmd.NewRequest([]string{"command", "spinner"}),
		Container: nil,
		Out:       terminal.New(),
	})

	assert.Empty(t, res)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCMDRun_Progress(t *testing.T) {
	setArgs(t, "command", "progress")

	output := testutil.StdoutOutputForFunc(main)

	assert.Contains(t, output, "\r1.000%")
	assert.Contains(t, output, "\r20.000%")
	assert.Contains(t, output, "\r50.000%")
	assert.Contains(t, output, "\r100.000%")
	assert.Contains(t, output, "Process Complete\n")
}

func TestCMDRun_ProgressContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create a proper context with logger to avoid nil pointer dereference
	container := &container.Container{
		Logger: logging.NewMockLogger(logging.ERROR),
	}

	res, err := progress(&gofr.Context{
		Context:       ctx,
		Request:       cmd.NewRequest([]string{"command", "progress"}),
		Container:     container,
		Out:           terminal.New(),
		ContextLogger: *logging.NewContextLogger(ctx, container.Logger),
	})

	assert.Empty(t, res)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestCMDRunWithInvalidCommand checks that an unregistered subcommand writes the
// "not a valid command" error to stderr, prints the help and exits with status 1.
// "help" is not a registered subcommand either; only -h/--help print help and exit 0.
func TestCMDRunWithInvalidCommand(t *testing.T) {
	testCases := []struct {
		arg     string
		wantErr string
	}{
		{"invalid", "'invalid' is not a valid command.\n"},
		{"help", "'help' is not a valid command.\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.arg, func(t *testing.T) {
			stdout, stderr, exitCode := runMainInSubprocess(t, tc.arg)

			assert.Equal(t, 1, exitCode, "exit code")
			assert.Equal(t, tc.wantErr, stderr, "stderr")
			assert.Contains(t, stdout, "Available commands:", "stdout")
		})
	}
}

// TestCMDRunWithEmptyParams tests the params command with empty name parameter
func TestCMDRunWithEmptyParams(t *testing.T) {
	expResp := "Hello !\n"
	setArgs(t, "command", "params", "-name=")
	output := testutil.StdoutOutputForFunc(main)

	assert.Contains(t, output, expResp, "TEST Failed.\n")
}

// TestCMDRunHelpCommand tests the help functionality
func TestCMDRunHelpCommand(t *testing.T) {
	testCases := []struct {
		args     []string
		expected []string
	}{
		{[]string{"command", "-h"}, []string{"Available commands:", "hello", "params", "spinner", "progress"}},
		{[]string{"command", "--help"}, []string{"Available commands:", "hello", "params", "spinner", "progress"}},
	}

	for i, tc := range testCases {
		setArgs(t, tc.args...)
		output := testutil.StdoutOutputForFunc(main)

		for _, expected := range tc.expected {
			assert.Contains(t, output, expected, "TEST[%d] Failed. Expected to contain: %s\n", i, expected)
		}
	}
}

// TestCMDRunHelpForSpecificCommand tests help for specific commands
func TestCMDRunHelpForSpecificCommand(t *testing.T) {
	testCases := []struct {
		args     []string
		expected string
	}{
		{[]string{"command", "hello", "-h"}, "hello world option"},
		{[]string{"command", "hello", "--help"}, "hello world option"},
	}

	for i, tc := range testCases {
		setArgs(t, tc.args...)
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, tc.expected, "TEST[%d] Failed.\n", i)
	}
}
