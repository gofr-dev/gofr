package main

import (
	"os"
	"strings"
	"testing"

	"gofr.dev/pkg/gofr/testutil"
)

// TestCMDRunWithNoArg checks that if no subcommand is found then error comes on stderr.
func TestCMDRunWithNoArg(t *testing.T) {
	expectedError := "No Command Found!"
	output := testutil.StderrOutputForFunc(main)
	if output != expectedError {
		t.Errorf("Expected: %s\n Got: %s", expectedError, output)
	}
}

func TestCMDRunWithProperArg(t *testing.T) {
	expectedOutput := "Hello World!"
	os.Args = []string{"command", "hello"}
	output := testutil.StdoutOutputForFunc(main)
	if output != expectedOutput {
		t.Errorf("Expected: %s\n Got: %s", expectedOutput, output)
	}
}

func TestCMDRunWithParams(t *testing.T) {
	expectedOutput := "Hello Vikash!"

	testcases := []string{
		"command params -name=Vikash",
		"command params   -name=Vikash",
		"command -name=Vikash params",
	}

	for _, tc := range testcases {
		os.Args = strings.Split(tc, " ")
		output := testutil.StdoutOutputForFunc(main)
		if output != expectedOutput {
			t.Errorf("Expected: %s\n Got: %s", expectedOutput, output)
		}
	}
}
