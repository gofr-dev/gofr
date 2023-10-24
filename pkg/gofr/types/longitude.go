package types

import (
	"gofr.dev/pkg/errors"
)

// Longitude denotes the longitude information and provides functionality to validate it
type Longitude float64

// Check validates the Longitude value within the valid range of -180 to 180 degrees.
// If the value is outside this range, it returns an InvalidParam error for "lng".
func (l *Longitude) Check() error {
	if *l > 180 || *l < -180 {
		return errors.InvalidParam{Param: []string{"lng"}}
	}

	return nil
}
