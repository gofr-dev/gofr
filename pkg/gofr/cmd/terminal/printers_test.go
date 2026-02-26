package terminal

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutput_Printf(t *testing.T) {
	var buf bytes.Buffer

	output := Out{out: &buf}

	format := "Hello, %s!"
	args := "world"
	output.Printf(format, args)

	expectedString := fmt.Sprintf(format, args)
	assert.Equal(t, expectedString, buf.String(), "Printf: unexpected written string. Expected: %s, got: %s", expectedString, buf.String())
}

func TestOutput_Print(t *testing.T) {
	var buf bytes.Buffer

	output := Out{out: &buf}

	message := "Hello, world!"
	output.Print(message)

	assert.Equal(t, message, buf.String(), "Printf: unexpected written string. Expected: %s, got: %s", message, buf.String())
}

func TestOutput_Println(t *testing.T) {
	var buf bytes.Buffer

	output := Out{out: &buf}

	message := "Hello, world!"
	output.Println(message)

	expectedString := message + "\n"
	assert.Equal(t, expectedString, buf.String(), "Printf: unexpected written string. Expected: %s, got: %s", expectedString, buf.String())
}
