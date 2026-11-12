package file

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanString(t *testing.T) {
	multiSpace := "  read   file \n\t done  "

	tests := []struct {
		name   string
		input  *string
		expOut string
	}{
		{name: "nil input returns empty string", input: nil, expOut: ""},
		{name: "whitespace is collapsed and trimmed", input: &multiSpace, expOut: "read file done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expOut, cleanString(tt.input))
		})
	}
}
