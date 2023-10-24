package types

import (
	"unicode"

	"gofr.dev/pkg/errors"
)

// Enum represents an enum data type with a set of valid values and a current value.
//
// It is designed to define and manage enum values. The Enum type has three fields:
// - ValidValues: A slice of strings representing all the valid enum values.
// - Value: A string field holding the current enum value.
// - Parameter: A string field that denotes the parameter name associated with the enum values.
//
// This type is useful for scenarios where you want to work with predefined, discrete choices.
// It allows you to specify the valid values for an enum and track the current value.
type Enum struct {
	ValidValues []string `json:"validValues"`
	Value       string   `json:"value"`
	Parameter   string
}

// Check validates if the Value field of the Enum conforms to one of the valid enumeration values.
// It checks if the provided Value is in UPPER_SNAKE format (uppercase letters and underscores) and matches one of the ValidValues.
// If the Value doesn't match any of the valid values or doesn't conform to the format, it returns an InvalidParam error.
func (e Enum) Check() error {
	// Enum values MUST be in the UPPER_SNAKE format
	for _, v := range e.Value {
		// allow underscores and numbers as enum values
		if v == 95 || unicode.IsDigit(v) {
			continue
		} else if !unicode.IsLetter(v) || !unicode.IsUpper(v) {
			return errors.InvalidParam{Param: []string{e.Parameter}}
		}
	}

	for _, v := range e.ValidValues {
		if v == e.Value {
			return nil
		}
	}

	return errors.InvalidParam{Param: []string{e.Parameter}}
}
