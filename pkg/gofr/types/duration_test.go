package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestDuration_Check(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		err      error
	}{
		{"empty string passed for duration", "", errors.InvalidParam{Param: []string{"duration"}}},
		{"incorrect format for duration", "11Tss1M", errors.InvalidParam{Param: []string{"duration"}}},
		{"correct format for duration with minute", "P12Y4MT15M", nil},
		{"correct format with fractional", "P5Y", nil},
		{"smallest value used as decimal", "P0.5Y", nil},
		{"PnW", "P1W", nil},
		{"Incorrect duration", "PT", errors.InvalidParam{Param: []string{"duration"}}},
		{"Incorrect duration", "P", errors.InvalidParam{Param: []string{"duration"}}},
		{"Incorrect format", "PT1Y", errors.InvalidParam{Param: []string{"duration"}}},
		{"Invalid letters", "KP", errors.InvalidParam{Param: []string{"duration"}}},
		{"Invalid format", "1P1T", errors.InvalidParam{Param: []string{"duration"}}},
		{"Invalid format", "1P1Y", errors.InvalidParam{Param: []string{"duration"}}},
		{"Incorrect value", "P1K1", errors.InvalidParam{Param: []string{"duration"}}},
	}
	for i, tt := range tests {
		tt := tt

		err := Validate(tt.duration)

		assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n%s", i, tt.name)
	}
}
