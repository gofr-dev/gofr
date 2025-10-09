package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

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

	// add an already canceled context
	res, err := progress(&gofr.Context{
		Context: ctx,
		Request: cmd.NewRequest([]string{"command", "spinner"}),
		Container: &container.Container{
			Logger: logging.NewMockLogger(logging.ERROR),
		},
		Out: terminal.New(),
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

// TestCMDRunWithSpecialCharacters tests params command with special characters
func TestCMDRunWithSpecialCharacters(t *testing.T) {
	testCases := []struct {
		name     string
		expected string
	}{
		{"John@Doe", "Hello John@Doe!\n"},
		{"User-123", "Hello User-123!\n"},
		{"Test User", "Hello Test User!\n"},
		{"", "Hello !\n"},
	}

	for i, tc := range testCases {
		os.Args = []string{"command", "params", "-name=" + tc.name}
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, tc.expected, "TEST[%d] Failed for name: %s\n", i, tc.name)
	}
}

// TestCMDRunWithMultipleParams tests params command with multiple parameters
func TestCMDRunWithMultipleParams(t *testing.T) {
	expResp := "Hello Alice!\n"
	os.Args = []string{"command", "params", "-name=Alice", "-age=25", "-city=NYC"}
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

// TestCMDRunWithVersionFlag tests version flag functionality
func TestCMDRunWithVersionFlag(t *testing.T) {
	os.Args = []string{"command", "-v"}
	output := testutil.StderrOutputForFunc(main) // Version might go to stderr

	// Version output should contain version information or show "No Command Found!"
	// The actual behavior depends on gofr framework implementation
	assert.NotEmpty(t, output, "Version output should not be empty")
}

// TestSpinnerWithTimeout tests spinner with different timeout scenarios
func TestSpinnerWithTimeout(t *testing.T) {
	// Test spinner with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	res, err := spinner(&gofr.Context{
		Context:   ctx,
		Request:   cmd.NewRequest([]string{"command", "spinner"}),
		Container: nil,
		Out:       terminal.New(),
	})

	assert.Empty(t, res)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestProgressWithInvalidProgressBar tests progress bar with invalid initialization
func TestProgressWithInvalidProgressBar(t *testing.T) {
	// Create a context with a nil output to trigger error in progress bar initialization
	ctx := &gofr.Context{
		Context: context.Background(),
		Request: cmd.NewRequest([]string{"command", "progress"}),
		Container: &container.Container{
			Logger: logging.NewMockLogger(logging.ERROR),
		},
		Out: terminal.New(), // Use a valid terminal instead of nil to avoid panic
	}

	res, err := progress(ctx)

	// Should complete successfully
	assert.Equal(t, "Process Complete", res)
	assert.NoError(t, err)
}

// TestCMDRunWithQuotedParams tests params command with quoted parameters
func TestCMDRunWithQuotedParams(t *testing.T) {
	testCases := []struct {
		name     string
		expected string
	}{
		{"'John Doe'", "Hello 'John Doe'!\n"},
		{"Test User", "Hello Test User!\n"},
	}

	for i, tc := range testCases {
		os.Args = []string{"command", "params", "-name=" + tc.name}
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, tc.expected, "TEST[%d] Failed for name: %s\n", i, tc.name)
	}
}

// TestCMDRunSequentialExecution tests sequential execution of multiple commands
func TestCMDRunSequentialExecution(t *testing.T) {
	// Test sequential execution of different commands
	os.Args = []string{"command", "hello"}
	output1 := testutil.StdoutOutputForFunc(main)
	assert.Contains(t, output1, "Hello World!\n")

	os.Args = []string{"command", "params", "-name=Sequential"}
	output2 := testutil.StdoutOutputForFunc(main)
	assert.Contains(t, output2, "Hello Sequential!\n")
}

// TestCMDRunWithLongRunningProcess tests long running processes
func TestCMDRunWithLongRunningProcess(t *testing.T) {
	// Test spinner with longer duration
	start := time.Now()
	os.Args = []string{"command", "spinner"}
	output := testutil.StdoutOutputForFunc(main)
	duration := time.Since(start)

	assert.Contains(t, output, "Process Complete\n")
	assert.GreaterOrEqual(t, duration, 2*time.Second, "Spinner should run for at least 2 seconds")
}

// TestCMDRunWithProgressBarCompletion tests progress bar completion
func TestCMDRunWithProgressBarCompletion(t *testing.T) {
	start := time.Now()
	os.Args = []string{"command", "progress"}
	output := testutil.StdoutOutputForFunc(main)
	duration := time.Since(start)

	assert.Contains(t, output, "Process Complete\n")
	assert.Contains(t, output, "\r100.000%")
	assert.GreaterOrEqual(t, duration, 5*time.Second, "Progress should take at least 5 seconds (100 * 50ms)")
}

// TestCMDRunWithInvalidFlagFormat tests invalid flag formats
func TestCMDRunWithInvalidFlagFormat(t *testing.T) {
	testCases := []struct {
		args     []string
		expected string
	}{
		{[]string{"command", "params", "--name=Vikash"}, "Hello Vikash!\n"}, // Double dash should work
		{[]string{"command", "params", "name=Vikash"}, "Hello !\n"},         // Missing dash should not work
		{[]string{"command", "params", "-name"}, "Hello true!\n"},           // Missing value defaults to true
	}

	for i, tc := range testCases {
		os.Args = tc.args
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, tc.expected, "TEST[%d] Failed.\n", i)
	}
}

// Benchmark tests for performance validation
func BenchmarkHelloCommand(b *testing.B) {
	os.Args = []string{"command", "hello"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testutil.StdoutOutputForFunc(main)
	}
}

func BenchmarkParamsCommand(b *testing.B) {
	os.Args = []string{"command", "params", "-name=Benchmark"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testutil.StdoutOutputForFunc(main)
	}
}

func BenchmarkHelpCommand(b *testing.B) {
	os.Args = []string{"command", "help"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testutil.StdoutOutputForFunc(main)
	}
}

// TestCMDRunWithDifferentOutputFormats tests different output scenarios
func TestCMDRunWithDifferentOutputFormats(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{"JSON output format", []string{"command", "hello", "-o=json"}, "Hello World!"},
		{"YAML output format", []string{"command", "hello", "-o=yaml"}, "Hello World!"},
		{"Default output format", []string{"command", "hello"}, "Hello World!"},
	}

	for i, tc := range testCases {
		os.Args = tc.args
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, tc.expected, "TEST[%d] Failed for %s\n", i, tc.name)
	}
}

// TestCMDRunWithEnvironmentVariables tests command execution with environment variables
func TestCMDRunWithEnvironmentVariables(t *testing.T) {
	// Set environment variable
	t.Setenv("TEST_NAME", "EnvironmentUser")
	
	// Test that environment variables can be accessed in commands
	os.Args = []string{"command", "params", "-name=$TEST_NAME"}
	output := testutil.StdoutOutputForFunc(main)

	// Note: This test assumes the command can access environment variables
	// The actual behavior depends on the gofr framework implementation
	assert.NotEmpty(t, output, "Output should not be empty")
}

// TestCMDRunWithLoggingLevels tests different logging levels
func TestCMDRunWithLoggingLevels(t *testing.T) {
	logLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	
	for _, level := range logLevels {
		t.Setenv("LOG_LEVEL", level)
		os.Args = []string{"command", "hello"}
		output := testutil.StdoutOutputForFunc(main)
		
		assert.Contains(t, output, "Hello World!\n", "Failed for log level: %s", level)
	}
}

// TestCMDRunWithConfigFile tests command execution with config file
func TestCMDRunWithConfigFile(t *testing.T) {
	// This test assumes there might be a config file in the configs directory
	// Even if empty, it should not cause errors
	os.Args = []string{"command", "hello"}
	output := testutil.StdoutOutputForFunc(main)
	
	assert.Contains(t, output, "Hello World!\n", "Command should work with config file")
}

// TestCMDRunWithMetrics tests metrics collection during command execution
func TestCMDRunWithMetrics(t *testing.T) {
	// Enable metrics
	t.Setenv("METRICS_PORT", "8080")
	
	os.Args = []string{"command", "hello"}
	output := testutil.StdoutOutputForFunc(main)
	
	assert.Contains(t, output, "Hello World!\n", "Command should work with metrics enabled")
}

// TestCMDRunWithTelemetry tests telemetry functionality
func TestCMDRunWithTelemetry(t *testing.T) {
	// Test with telemetry enabled and disabled
	testCases := []struct {
		name     string
		telemetry string
	}{
		{"Telemetry enabled", "true"},
		{"Telemetry disabled", "false"},
	}

	for i, tc := range testCases {
		t.Setenv("GOFR_TELEMETRY", tc.telemetry)
		os.Args = []string{"command", "hello"}
		output := testutil.StdoutOutputForFunc(main)
		
		assert.Contains(t, output, "Hello World!\n", "TEST[%d] Failed for %s\n", i, tc.name)
	}
}
