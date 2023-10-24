package types

import "gofr.dev/pkg/errors"

// Latitude represents latitude information as a floating-point value and offers functionality
// for validating its value.
//
// Latitude values are typically expressed in degrees and should fall within the valid range of
// -90 degrees (South) to 90 degrees (North). The Check method ensures that the provided Latitude
// value is within this range. If the value is outside this range, it returns an error with the "lat" parameter.
type Latitude float64

// Check validates if the Latitude value is within the valid range of -90 to 90 degrees.
//
// If the value is outside this range, it returns an InvalidParam error with the "lat" parameter.
func (l *Latitude) Check() error {
	if *l > 90 || *l < -90 {
		return errors.InvalidParam{Param: []string{"lat"}}
	}

	return nil
}
