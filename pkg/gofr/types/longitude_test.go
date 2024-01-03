package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestLongitude_Check(t *testing.T) {
	tests := []struct {
		name      string
		longitude Longitude
		err       error
	}{
		{"longitude value less than -180", -181.32, errors.InvalidParam{Param: []string{"lng"}}},
		{"longitude value larger than 180", 189.32, errors.InvalidParam{Param: []string{"lng"}}},
		{"correct longitude value", 89.99, nil},
		{"correct negative longitude value", -45.00, nil},
	}

	for i, tt := range tests {
		tt := tt

		err := Validate(&tt.longitude)

		assert.Equal(t, tt.err, err, "TEST[%d], Failed.\n", i)
	}
}
