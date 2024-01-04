package gofr

import (
	"testing"

	"gofr.dev/pkg/gofr/testutil"
)

func TestNewCMD(t *testing.T) {
	a := NewCMD()
	// Without args we should get error on stderr.
	outputWithoutArgs := testutil.StderrOutputForFunc(a.Run)
	if outputWithoutArgs != "No Command Found!" {
		t.Errorf("Stderr output mismatch. Got: %s ", outputWithoutArgs)
	}
}
