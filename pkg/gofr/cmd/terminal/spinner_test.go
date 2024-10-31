package terminal

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpinner(t *testing.T) {
	var waitTime = 3 * time.Second

	// Testing Dot spinner
	b := &bytes.Buffer{}
	out := &Out{out: b}
	spinner := NewDotSpinner(out)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(waitTime)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr := b.String()
	assert.NotZero(t, outputStr)

	// Testing Globe Spinner
	b = &bytes.Buffer{}
	out = &Out{out: b}
	spinner = NewGlobeSpinner(out)

	// Start the spinner
	spinner.Spin()

	// Let it run for a bit
	time.Sleep(waitTime)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr = b.String()
	assert.NotZero(t, outputStr)

	// Testing Pulse Spinner
	b = &bytes.Buffer{}
	out = &Out{out: b}
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
	assert.NotZero(t, outputStr)
}
