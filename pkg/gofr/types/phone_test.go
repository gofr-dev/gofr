package types

import (
	"reflect"
	"testing"

	"gofr.dev/pkg/errors"
)

func TestPhone_Check(t *testing.T) {
	tests := []struct {
		phone Phone
		err   error
	}{
		{"", errors.InvalidParam{Param: []string{"Phone Number length"}}},
		{"+", errors.InvalidParam{Param: []string{"Phone Number contains Non Numeric characters"}}},
		{"123455678901234556", errors.InvalidParam{Param: []string{"Phone Number length"}}},
		{"1234567890", errors.InvalidParam{Param: []string{"Phone Number doesn't contain + char"}}},
		{"+17777777777", nil},
		{"+12345589abc", errors.InvalidParam{Param: []string{"Phone Number contains Non Numeric characters"}}},
		{"+-2345589", errors.InvalidParam{Param: []string{"Phone Number contains Non Numeric characters"}}},
		{"+61491570156", nil},
		{"+134", errors.InvalidParam{Param: []string{"Phone Number"}}},
		{"+912123456789098", nil},
	}
	for i, tt := range tests {
		tt := tt

		err := Validate(tt.phone)
		if !reflect.DeepEqual(err, tt.err) {
			t.Errorf("[TEST ID %d]Got %v\tExpected %v", i+1, err, tt.err)
		}
	}
}
