package terminal

import (
	"bytes"
	"fmt"
	"testing"
)

func TestOutput_Printf(t *testing.T) {
	var buf bytes.Buffer
	output := Output{out: &buf}

	format := "Hello, %s!"
	args := "world"
	output.Printf(format, args)

	expectedString := fmt.Sprintf(format, args)
	if buf.String() != expectedString {
		t.Errorf("Printf: unexpected written string. Expected: %s, got: %s", expectedString, buf.String())
	}
}

func TestOutput_Print(t *testing.T) {
	var buf bytes.Buffer
	output := Output{out: &buf}

	message := "Hello, world!"
	output.Print(message)

	if buf.String() != message {
		t.Errorf("Print: unexpected written string. Expected: %s, got: %s", message, buf.String())
	}
}

func TestOutput_Println(t *testing.T) {
	var buf bytes.Buffer
	output := Output{out: &buf}

	message := "Hello, world!"
	output.Println(message)

	expectedString := message + "\n"
	if buf.String() != expectedString {
		t.Errorf("Println: unexpected written string. Expected: %s, got: %s", expectedString, buf.String())
	}
}
