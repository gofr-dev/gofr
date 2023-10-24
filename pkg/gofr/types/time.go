package types

import (
	"time"

	"gofr.dev/pkg/errors"
)

// Time denotes a time in string format
type Time string

// Check validates the Time value by parsing it using the expected format.
// It returns an error if the parsing fails, using the parameter name "time" in the error message.
func (t Time) Check() error {
	_, err := time.Parse("15:04:05.000", string(t))
	if err != nil {
		return errors.InvalidParam{Param: []string{"time"}}
	}

	return nil
}
