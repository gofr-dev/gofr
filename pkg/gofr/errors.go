package gofr

import "fmt"

// errOutOfRange denotes the errors that occur when a range in schedule is out of scope for the particular time unit.
type errOutOfRange struct {
	rangeVal interface{}
	input    string
	min, max int
}

func (e errOutOfRange) Error() string {
	return fmt.Sprintf("out of range for %v in %s. %v must be in range %d-%d",
		e.rangeVal, e.input, e.rangeVal, e.min, e.max)
}

type errParsing struct {
	invalidPart string
	base        string
}

func (e errParsing) Error() string {
	if e.base != "" {
		return fmt.Sprintf("unable to parse %s part in %s", e.invalidPart, e.base)
	}
	return fmt.Sprintf("unable to parse %s", e.invalidPart)
}
