package types

import (
	"reflect"
	"testing"

	"gofr.dev/pkg/errors"
)

func TestEmail_Check(t *testing.T) {
	tc := []struct {
		name        string
		email       Email
		expectedErr error
	}{
		{"incorrect format for email, string passed", "Something", errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"incorrect format for email", "abc@gmail++++yayy.com", errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"incorrect format for email", "12~~3@123...com", errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"correct format for email", "www@gmail.com", nil},
		{"valid email", "c.r@yahoo.com", nil},
		{"valid email", "c.r@yahoo.co.in", nil},
		{"incorrect format for email, string passed", "xy@98", errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"incorrect format for email, string passed", "xy@78.", errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"incorrect format for email, string passed", "xy@78.c", errors.InvalidParam{Param: []string{"emailAddress"}}},
		{"correct format for email, string passed", "xy@q.com", nil},
		{"correct format for email, string passed", "abcd@g.abcde.com", nil},
		{"correct format for email with non ascii characters", "añabcd@gmail.com", nil},
		{"incorrect format for email with whitespace", "añ abcd@gmail.com", errors.InvalidParam{Param: []string{"emailAddress"}}},
	}

	for _, c := range tc {
		c := c

		r := Validate(c.email)
		if !reflect.DeepEqual(r, c.expectedErr) {
			t.Errorf("%v Expected value: %v for %v got %v", c.name, c.expectedErr, c.email, r)
		}
	}
}
