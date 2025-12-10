package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd"
	"gofr.dev/pkg/gofr/cmd/terminal"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

// TestCMDRunWithNoArg checks that if no subcommand is found then error comes on stderr.
func TestCMDRunWithNoArg(t *testing.T) {
	expErr := "No Command Found!\n"
	output := testutil.StderrOutputForFunc(main)

	assert.Equal(t, expErr, output, "TEST Failed.\n")
}

func TestCMDRunWithProperArg(t *testing.T) {
	expResp := "Hello World!\n"
	os.Args = []string{"command", "hello"}

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
		os.Args = strings.Split(command, " ")
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, expResp, "TEST[%d], Failed.\n", i)
	}
}

func TestCMDRun_Spinner(t *testing.T) {
	os.Args = []string{"command", "spinner"}
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
	os.Args = []string{"command", "progress"}

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
		Context:   ctx,
		Request:   cmd.NewRequest([]string{"command", "progress"}),
		Container: container,
		Out:       terminal.New(),
	})

	assert.Empty(t, res)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestCMDRunWithInvalidCommand tests that invalid commands return appropriate error
func TestCMDRunWithInvalidCommand(t *testing.T) {
	expErr := "No Command Found!\n"
	os.Args = []string{"command", "invalid"}
	output := testutil.StderrOutputForFunc(main)

	assert.Equal(t, expErr, output, "TEST Failed.\n")
}

// TestCMDRunWithEmptyParams tests the params command with empty name parameter
func TestCMDRunWithEmptyParams(t *testing.T) {
	expResp := "Hello !\n"
	os.Args = []string{"command", "params", "-name="}
	output := testutil.StdoutOutputForFunc(main)

	assert.Contains(t, output, expResp, "TEST Failed.\n")
}

// TestCMDRunHelpCommand tests the help functionality
func TestCMDRunHelpCommand(t *testing.T) {
	testCases := []struct {
		args     []string
		expected []string
	}{
		{[]string{"command", "help"}, []string{"Available commands:", "hello", "params", "spinner", "progress"}},
		{[]string{"command", "-h"}, []string{"Available commands:", "hello", "params", "spinner", "progress"}},
		{[]string{"command", "--help"}, []string{"Available commands:", "hello", "params", "spinner", "progress"}},
	}

	for i, tc := range testCases {
		os.Args = tc.args
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
		os.Args = tc.args
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, tc.expected, "TEST[%d] Failed.\n", i)
	}
}
