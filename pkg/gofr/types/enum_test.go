package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestEnum_Check(t *testing.T) {
	testcases := []struct {
		valid []string
		value string
		err   error
	}{ // invalid cases
		{[]string{"X", "Y"}, "r", errors.InvalidParam{Param: []string{"abc"}}},
		{[]string{"X", "Y"}, "y", errors.InvalidParam{Param: []string{"abc"}}},
		{[]string{"X", "Y"}, "y@", errors.InvalidParam{Param: []string{"abc"}}},
		{[]string{"X", "Y"}, "K", errors.InvalidParam{Param: []string{"abc"}}},
		// valid cases
		{[]string{"X", "Y"}, "Y", nil},
		{[]string{"RED", "RED1"}, "RED1", nil},
		{[]string{"RED_BLUE", "RED_GREEN"}, "RED_GREEN", nil},
	}

	for i, v := range testcases {
		e := Enum{ValidValues: v.valid, Value: v.value, Parameter: "abc"}

		err := e.Check()

		assert.Equal(t, v.err, err, "TEST[%d], Failed.\n", i)
	}
}
