package errors

import (
	"fmt"
)

// Timeout is used when request timeout occurs
type Timeout struct {
	URL string
}

// Error returns an error message indicating that the request to URL has timed out
func (t Timeout) Error() string {
	return fmt.Sprintf("Request to %v has Timed out!", t.URL)
}
