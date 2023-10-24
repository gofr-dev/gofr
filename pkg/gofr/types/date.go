// Package types provides custom types and utility functions for handling various data types.
package types

import (
	"time"

	"gofr.dev/pkg/errors"
)

// Date represents a date in string format (e.g., "2006-01-02") and provides functionality
// to validate its format.
type Date string

// Check validates the Date value to ensure it conforms to the "2006-01-02" format.
// If the Date is not in the expected format, it returns an error specifying the "date" parameter.
func (d Date) Check() error {
	_, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return errors.InvalidParam{Param: []string{"date"}}
	}

	return nil
}
