package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestLatitude_Check(t *testing.T) {
	tests := []struct {
		latitude Latitude
		err      error
	}{
		{-97.32, errors.InvalidParam{Param: []string{"lat"}}},
		{97.32, errors.InvalidParam{Param: []string{"lat"}}},
		{89.99, nil},
		{-45.00, nil},
	}
	for i, tt := range tests {
		tt := tt

		err := Validate(&tt.latitude)

		assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n", i)
	}
}
