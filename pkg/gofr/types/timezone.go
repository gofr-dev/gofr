package types

import (
	"time"

	"gofr.dev/pkg/errors"
)

type TimeZone string

// Check validates the TimeZone value.
// It ensures that the value corresponds to a valid time zone.
func (t TimeZone) Check() error {
	_, err := time.LoadLocation(string(t))
	if err != nil {
		return errors.InvalidParam{Param: []string{"timeZone"}}
	}

	return nil
}
