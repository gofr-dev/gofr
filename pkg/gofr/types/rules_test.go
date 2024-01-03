package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		rules   []Rule
		wantErr error
	}{
		{"valid phone", []Rule{Phone("+17777777777")}, nil},
		{"invalid phone", []Rule{Phone("+17777777777qq")}, errors.InvalidParam{Param: []string{"Phone Number contains Non Numeric characters"}}},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.rules...)

			assert.Equal(t, tt.wantErr, err, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}
