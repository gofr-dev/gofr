package terminal

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestSpinner(t *testing.T) {
	var waitTime = 3

	// Testing Dot spinner
	b := &bytes.Buffer{}
	output := &Output{out: b}
	spinner := NewDotSpinner(output)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(time.Duration(waitTime) * time.Second)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr := b.String()
	if len(outputStr) == 0 {
		t.Error("No output received from spinner")
	}

	// Testing Globe Spinner
	b = &bytes.Buffer{}
	output = &Output{out: b}
	spinner = NewGlobeSpinner(output)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(time.Duration(waitTime) * time.Second)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr = b.String()
	if len(outputStr) == 0 {
		t.Error("No output received from spinner")
	}

	// Testing Pulse Spinner
	b = &bytes.Buffer{}
	output = &Output{out: b}
	spinner = NewPulseSpinner(output)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(time.Duration(waitTime) * time.Second)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr = b.String()
	fmt.Println(outputStr)
	if len(outputStr) == 0 {
		t.Error("No output received from spinner")
	}
}
