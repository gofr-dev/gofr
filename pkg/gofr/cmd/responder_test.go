package cmd

import (
	"errors"
	"testing"

	"github.com/vikash/gofr/pkg/gofr/testutil"
)

func TestResponder_Respond(t *testing.T) {
	r := Responder{}

	out := testutil.StdoutOutputForFunc(func() {
		r.Respond("data", nil)
	})

	if out != "data" {
		t.Errorf("Responder stdout output error. Expected: %s Got: %s", "data", out)
	}

	err := testutil.StderrOutputForFunc(func() {
		r.Respond(nil, errors.New("error")) //nolint:goerr113 // We are testing if a dynamic error would work.
	})

	if err != "error" {
		t.Errorf("Responder stderr output error. Expected: %s Got: %s", "error", out)
	}
}
