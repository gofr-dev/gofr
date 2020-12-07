package logging

import (
	"testing"

	"github.com/vikash/gofr/pkg/gofr/testutil"
)

const testLogStatement = "hello log!"

func TestLogger_Log(t *testing.T) {
	expectedLog := testLogStatement + "\n" // Note that Log always adds a new line.

	f := func() {
		logger := NewLogger(DEBUG)
		logger.Log(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	if output != expectedLog {
		t.Errorf("Stdout mismatch. Expected: %s Got: %s", expectedLog, output)
	}
}

func TestLogger_Logf(t *testing.T) {
	expectedLog := testLogStatement + "\n"
	f := func() {
		logger := NewLogger(DEBUG)
		logger.Logf("%s", testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	if output != expectedLog {
		t.Errorf("Stdout mismatch. Expected: %s Got: %s", expectedLog, output)
	}
}

func TestLogger_Error(t *testing.T) {
	expectedLog := testLogStatement + "\n" // Note that Error always adds a new line.

	f := func() {
		logger := NewLogger(DEBUG)
		logger.Error(testLogStatement)
	}

	output := testutil.StderrOutputForFunc(f)

	if output != expectedLog {
		t.Errorf("Stdout mismatch. Expected: %s Got: %s", expectedLog, output)
	}
}

func TestLogger_Errorf(t *testing.T) {
	expectedLog := testLogStatement + "\n"
	f := func() {
		logger := NewLogger(DEBUG)
		logger.Errorf("%s", testLogStatement)
	}

	output := testutil.StderrOutputForFunc(f)

	if output != expectedLog {
		t.Errorf("Stdout mismatch. Expected: %s Got: %s", expectedLog, output)
	}
}
