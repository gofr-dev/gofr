package terminal

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewOutput(t *testing.T) {
	// intialize a new standard output stream
	o := New()

	assert.Equal(t, os.Stdout, o.out)
	assert.Equal(t, uintptr(1), o.fd)

	// for tests, the os.Stdout do not directly outputs to the terminal
	assert.Equal(t, false, o.isTerminal)
}
