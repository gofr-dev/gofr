package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

// TestCMDRunWithNoArg checks that if no subcommand is found then error comes on stderr.
func TestCMDRunWithNoArg(t *testing.T) {
	expErr := "No Command Found!"
	output := testutil.StderrOutputForFunc(main)

	assert.Equal(t, output, expErr, "TEST Failed.\n")
}

func TestCMDRunWithProperArg(t *testing.T) {
	expResp := "Hello World!"
	os.Args = []string{"command", "-hello"}

	output := testutil.StdoutOutputForFunc(main)

	assert.Contains(t, output, expResp, "TEST Failed.\n")
}

func TestCMDRunWithParams(t *testing.T) {
	expResp := "Hello Vikash!"

	commands := []string{
		"command -params name=Vikash",
		"command -params   name=Vikash",
		"command -params name=Vikash -",
	}

	for i, command := range commands {
		os.Args = strings.Split(command, " ")
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, expResp, "TEST[%d], Failed.\n", i)
	}
}

func TestHelperFunction(t *testing.T) {
	expResp := "usage: command [-h|--help] [-hello|--hello-world] [-params]"

	commands := []string{
		"command -h",
		"command --help",
		"command   --help",
		"command  -h",
	}

	for i, command := range commands {
		os.Args = strings.Split(command, " ")
		output := testutil.StdoutOutputForFunc(main)

		assert.Contains(t, output, expResp, "TEST[%d], Failed.\n", i)
	}
}
