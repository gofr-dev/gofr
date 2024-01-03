package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestDate_Check(t *testing.T) {
	tests := []struct {
		name string
		date Date
		err  error
	}{
		{"date value empty string", "", errors.InvalidParam{Param: []string{"date"}}},
		{"date value not correct format", "12", errors.InvalidParam{Param: []string{"date"}}},
		{"correct date value", "2018-10-11", nil},
		{"date format incorrect", "2018-10-1", errors.InvalidParam{Param: []string{"date"}}},
		{"date format incorrect datetime", "2018-10-01 10-05-02", errors.InvalidParam{Param: []string{"date"}}},
	}

	for i, tt := range tests {
		tt := tt

		err := Validate(tt.date)

		assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n%s", i, tt.name)
	}
}
