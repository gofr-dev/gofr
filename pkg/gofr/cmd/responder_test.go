package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func TestResponder_Respond(t *testing.T) {
	r := Responder{}

	out := testutil.StdoutOutputForFunc(func() {
		r.Respond("data", nil)
	})

	err := testutil.StderrOutputForFunc(func() {
		r.Respond(nil, errors.New("error")) //nolint:err113 // We are testing if a dynamic error would work.
	})

	assert.Equal(t, "data\n", out, "TEST Failed.\n", "Responder stdout output")

	assert.Equal(t, "error\n", err, "TEST Failed.\n", "Responder stderr output")
}
