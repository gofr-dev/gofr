package types

import (
	"gofr.dev/pkg/errors"
)

// Location denotes a location with the lat, long values
type Location struct {
	// Latitude denotes the latitude of a location
	Latitude *Latitude `json:"lat"`
	// Longitude denotes the longitude of a location
	Longitude *Longitude `json:"lng"`
}

// Check validates the Location struct fields.
// If any validation fails, it returns an InvalidParam error for the corresponding field.
func (l Location) Check() error {
	if l.Latitude == nil && l.Longitude == nil {
		return errors.MultipleErrors{Errors: []error{errors.InvalidParam{Param: []string{"lat is nil"}},
			errors.InvalidParam{Param: []string{"lng is nil"}}}}
	}

	if l.Latitude == nil {
		return errors.InvalidParam{Param: []string{"lat is nil"}}
	}

	err := Validate(l.Latitude)
	if err != nil {
		return errors.InvalidParam{Param: []string{"lat"}}
	}

	if l.Longitude == nil {
		return errors.InvalidParam{Param: []string{"lng is nil"}}
	}

	err = Validate(l.Longitude)
	if err != nil {
		return errors.InvalidParam{Param: []string{"lng"}}
	}

	return nil
}
