package types

import (
	"regexp"

	"gofr.dev/pkg/errors"
)

// this will compile the regex once instead of compiling it each time when it is being called.
var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~[:^ascii:]-]+@(?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9]\\.)+[a-zA-Z]{2,}$")

// Email represents an email address in string format and offers functionality to validate its format.
//
// Email addresses are expected to conform to the typical email format, and the Check method utilizes
// a regular expression to verify whether the provided string matches the expected email address format.
//
// If the input Email string does not adhere to the email format, the Check method returns an error
// with the "emailAddress" parameter, indicating that the email is invalid.
type Email string

// Check validates if the Email string conforms to a valid email address format.
// It uses a regular expression to check if the provided string matches the expected email format.
// If the input Email string doesn't match the email format, it returns an InvalidParam error.
func (e Email) Check() error {
	if !emailRegex.MatchString(string(e)) {
		return errors.InvalidParam{Param: []string{"emailAddress"}}
	}

	return nil
}
