package terminal

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestSpinner(t *testing.T) {
	var waitTime = 3 * time.Second

	// Testing Dot spinner
	b := &bytes.Buffer{}
	out := &output{out: b}
	spinner := NewDotSpinner(out)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(waitTime)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr := b.String()
	if len(outputStr) == 0 {
		t.Error("No output received from spinner")
	}

	// Testing Globe Spinner
	b = &bytes.Buffer{}
	out = &output{out: b}
	spinner = NewGlobeSpinner(out)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(waitTime)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr = b.String()
	if len(outputStr) == 0 {
		t.Error("No output received from spinner")
	}

	// Testing Pulse Spinner
	b = &bytes.Buffer{}
	out = &output{out: b}
	spinner = NewPulseSpinner(out)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(waitTime)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr = b.String()
	fmt.Println(outputStr)
	if len(outputStr) == 0 {
		t.Error("No output received from spinner")
	}
}
