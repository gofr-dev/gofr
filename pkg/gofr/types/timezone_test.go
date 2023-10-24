package types

import (
	"reflect"
	"testing"

	"gofr.dev/pkg/errors"
)

func TestTimeZone_Check(t *testing.T) {
	tc := []struct {
		name        string
		timezone    TimeZone
		expectedErr error
	}{
		{"correct timezone UTC passed", "UTC", nil},
		{"correct timezone Local passed", "Local", nil},
		{"correct format for timezone", "America/New_York", nil},
		{"incorrect format for timezone", "warzone", errors.InvalidParam{Param: []string{"timeZone"}}},
	}

	for _, c := range tc {
		c := c

		r := Validate(c.timezone)
		if !reflect.DeepEqual(r, c.expectedErr) {
			t.Errorf("%v Expected value: for %v got %v", c.name, c.expectedErr, c.timezone)
		}
	}
}
