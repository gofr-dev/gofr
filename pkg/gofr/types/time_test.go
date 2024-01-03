package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestTime_Check(t *testing.T) {
	tests := []struct {
		name      string
		timecheck Time
		err       error
	}{
		{"correct time format passed ", "09:55:23.198", nil},
		{"correct time format for 24 hours check", "23:55:23.198", nil},
		{"time with no fraction of seconds.", "23:44:00", errors.InvalidParam{Param: []string{"time"}}},
		{"incorrect datetime passed as time", "2018-10-01 10-05-02", errors.InvalidParam{Param: []string{"time"}}},
	}
	for _, tt := range tests {
		tt := tt

		err := Validate(tt.timecheck)

		assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n")
	}
}
