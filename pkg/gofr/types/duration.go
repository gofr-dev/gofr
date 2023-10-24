package types

import (
	"regexp"

	"gofr.dev/pkg/errors"
)

//nolint:lll // this will compile the regex once instead of compiling it each time when it is being called.
var durationRegex = regexp.MustCompile(`^P((((\d+(\.\d+)?)Y)?((\d+(\.\d+)?)M)?((\d+(\.\d+)?)D)?)?(T((\d+(\.\d+)?)H)?((\d+(\.\d+)?)M)?((\d+(\.\d+)?)S)?)?){1}$|(^P(\d+(\.\d+)?)W$)`)

// Duration represents a duration string and provides functionality for validating its format.
//
// A duration in ISO 8601 format consists of components for years (Y), months (M), days (D),
// hours (H), minutes (M), seconds (S), and weeks (W). The format can be written as PnYnMnDTnHnMnS or PnW.
//
// Duration values should adhere to this ISO 8601 format, and the Check method validates the provided
// Duration string to ensure it conforms to the expected format. If the provided Duration does not meet
// the format criteria, the method returns an error with the "duration" parameter.
type Duration string

// Check validates if the Duration string conforms to the expected format.
//
// The ISO 8601 duration format consists of components for years (Y), months (M), days (D),
// hours (H), minutes (M), and seconds (S), as well as weeks (W).
// The format can be written as PnYnMnDTnHnMnS or PnW.
func (d Duration) Check() error {
	// The format for duration is :PnYnMnDTnHnMnS or PnW
	const durationLen = 3
	if len(d) < durationLen {
		return errors.InvalidParam{Param: []string{"duration"}}
	}

	matches := durationRegex.FindStringSubmatch(string(d))

	if len(matches) == 0 {
		return errors.InvalidParam{Param: []string{"duration"}}
	}

	return nil
}
