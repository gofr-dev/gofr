package terminal

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOut_SetColor(t *testing.T) {
	tests := []struct {
		name      string
		colorCode int
	}{
		{"black", Black},
		{"red", Red},
		{"green", Green},
		{"white", White},
		{"bright white", BrightWhite},
	}

	for _, tc := range tests {
		var buf bytes.Buffer

		output := Out{out: &buf}
		output.SetColor(tc.colorCode)

		expected := fmt.Sprintf(csi+"38;5;%dm", tc.colorCode)
		assert.Equal(t, expected, buf.String(), "TEST[%s] Failed.\n", tc.name)
	}
}

func TestOut_ResetColor(t *testing.T) {
	var buf bytes.Buffer

	output := Out{out: &buf}
	output.ResetColor()

	expected := csi + "0m"
	assert.Equal(t, expected, buf.String(), "TestOut_ResetColor Failed.\n")
}
