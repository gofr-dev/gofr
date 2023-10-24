package types

import (
	"reflect"
	"testing"

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
		if !reflect.DeepEqual(err, tt.err) {
			t.Errorf("[TESTCASE %d]Failed. Got :%v\tExpected: %v", i+1, err, tt.err)
		}
	}
}
