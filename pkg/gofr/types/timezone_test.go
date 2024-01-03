package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

	for i, c := range tc {
		c := c

		r := Validate(c.timezone)

		assert.Equal(t, c.expectedErr, r, "TEST[%d], Failed.\n%s", i, c.name)
	}
}
