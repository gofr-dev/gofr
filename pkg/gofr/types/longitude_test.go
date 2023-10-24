package types

import (
	"reflect"
	"testing"

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
	for _, tt := range tests {
		tt := tt

		err := Validate(&tt.longitude)
		if !reflect.DeepEqual(err, tt.err) {
			t.Errorf("%v, Failed. Got :%v\tExpected: %v", tt.name, err, tt.err)
		}
	}
}
