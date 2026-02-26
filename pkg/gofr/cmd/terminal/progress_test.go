package terminal

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressBar_SuccessCases(t *testing.T) {
	total := int64(100)

	var out bytes.Buffer

	stream := &Out{terminal{isTerminal: true, fd: 1}, &out}
	bar := ProgressBar{
		stream:  stream,
		current: 0,
		total:   100,
		mu:      sync.Mutex{},
	}

	// Mock terminal size
	bar.tWidth = 120

	// Increment the progress bar
	bar.Incr(10)

	// Verify the output
	expectedOutput := "\r[████░                                             ] 10.000%"
	assert.Equal(t, expectedOutput, out.String())

	// Increment the progress bar to completion
	bar.Incr(total - 10)

	// Verify the completion output
	expectedCompletion := "\r[████░                                             ] 10.000%\r" +
		"[██████████████████████████████████████████████████] 100.000%\n"
	assert.Equal(t, expectedCompletion, out.String())
}

func TestProgressBar_Fail(t *testing.T) {
	var out bytes.Buffer

	stream := &Out{terminal{isTerminal: true, fd: 1}, &out}
	bar, err := NewProgressBar(stream, int64(-1))

	require.Error(t, err)
	require.ErrorIs(t, err, errTermSize)
	assert.NotNil(t, bar)
}

func TestProgressBar_Incr(t *testing.T) {
	var out bytes.Buffer

	stream := &Out{terminal{isTerminal: true, fd: 1}, &out}
	bar := ProgressBar{stream: stream, current: 0, total: 100, mu: sync.Mutex{}}
	// doing this as while calculating terminal size, the code will not
	// be able to determine its width since we are not attaching an actual
	// terminal for testing
	bar.tWidth = 120

	// Increment the progress by 20
	b := bar.Incr(int64(20))
	expectedOut := "\r[█████████░                                        ] 20.000%"

	assert.True(t, b)
	assert.Equal(t, int64(20), bar.current)
	assert.Equal(t, expectedOut, out.String())

	// increment the progress by 100 units.
	b = bar.Incr(int64(100))
	expectedOut = "\r[█████████░                                        ] 20.000%\r" +
		"[██████████████████████████████████████████████████] 100.000%\n"

	assert.False(t, b)
	assert.Equal(t, expectedOut, out.String())
	assert.Equal(t, int64(100), bar.current)
}

func TestProgressBar_getString(t *testing.T) {
	testCases := []struct {
		desc        string
		current     int64
		tWidth      int
		total       int64
		expectedOut string
	}{
		{
			desc:        "current and total negative",
			current:     -1,
			total:       -1,
			tWidth:      120,
			expectedOut: "",
		},
		{
			desc:        "terminal width < 110",
			current:     20,
			total:       100,
			expectedOut: "20.000%",
		},
		{
			desc:        "0% progress, 50 spaces",
			tWidth:      120,
			current:     0,
			total:       100,
			expectedOut: "[                                                  ] 0.000%",
		},
		{
			desc:        "100% progress",
			tWidth:      120,
			current:     100,
			total:       100,
			expectedOut: "[██████████████████████████████████████████████████] 100.000%",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := ProgressBar{
				current: tc.current,
				total:   tc.total,
				tWidth:  tc.tWidth,
			}

			out := p.getString()

			assert.Equal(t, tc.expectedOut, out)
		})
	}
}
