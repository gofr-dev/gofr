package terminal

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpinner(t *testing.T) {
	var (
		waitTime = 1 * time.Second
		ctx      = context.TODO()
	)

	// Testing Dot spinner
	b := &bytes.Buffer{}
	out := &Out{out: b}
	spinner := NewDotSpinner(out)

	// Start the spinner
	spinner.Spin(ctx)

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
	spinner.Spin(ctx)

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
	spinner.Spin(ctx)

	// Let it run for a bit
	time.Sleep(waitTime)

	// Stop the spinner
	spinner.Stop()

	// Check if output contains spinner frames
	outputStr = b.String()
	fmt.Println(outputStr)
	assert.NotZero(t, outputStr)
}

func TestSpinner_contextDone(t *testing.T) {
	b := &bytes.Buffer{}
	out := &Out{out: b}
	spinner := NewDotSpinner(out)
	ctx, cancel := context.WithCancel(context.Background())

	// start the spinner
	spinner.Spin(ctx)

	// let the spinner start spinning
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-spinner.ticker.C:
		t.Error("ticker should have been stopped after cancel")
	case <-time.After(1 * time.Second):
		// successful case as ticker did not send a tick
		return
	}
}
