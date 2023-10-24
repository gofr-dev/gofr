package types

import (
	"regexp"
	"strconv"

	"gofr.dev/pkg/errors"
)

// Phone denotes the phone number in string format and provides functionality to validate it
type Phone string

// this will compile the regex once instead of compiling it each time when it is being called.
var phoneRegex = regexp.MustCompile(`^\+[1-9]{1,3}\d{3,13}$`)

// Check validates the Phone number in E.164 format, considering length, "+" presence, and non-numeric characters.
// It returns an InvalidParam error on failure.
func (p Phone) Check() error {
	// Phone numbers MUST be formatted in the E.164 format.
	// E.164 is the following format: "+" + <country code> + <subscriber number with area code>.
	if len(string(p)) > 16 || p == "" {
		return errors.InvalidParam{Param: []string{"Phone Number length"}}
	}

	if p[0] != '+' {
		return errors.InvalidParam{Param: []string{"Phone Number doesn't contain + char"}}
	}

	phoneVal, err := strconv.Atoi(string(p[1:]))
	if err != nil || phoneVal < 0 {
		return errors.InvalidParam{Param: []string{"Phone Number contains Non Numeric characters"}}
	}

	if !phoneRegex.MatchString(string(p)) {
		return errors.InvalidParam{Param: []string{"Phone Number"}}
	}

	return nil
}
