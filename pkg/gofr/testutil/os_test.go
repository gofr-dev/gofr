package testutil

import (
	"fmt"
	"os"
	"testing"
)

func TestStdoutOutputForFunc(t *testing.T) {
	msg := "Hello Stdout!"

	out := StdoutOutputForFunc(func() {
		fmt.Fprint(os.Stdout, msg)
	})

	if out != msg {
		t.Errorf("Expected: %s got: %s", msg, out)
	}
}

func TestStderrOutputForFunc(t *testing.T) {
	msg := "Hello Stderr!"

	out := StderrOutputForFunc(func() {
		fmt.Fprint(os.Stderr, msg)
	})

	if out != msg {
		t.Errorf("Expected: %s got: %s", msg, out)
	}
}
