package errors

import "fmt"

// ForbiddenRequest is used when an incoming request is refused authorization
type ForbiddenRequest struct {
	URL string
}

// Error returns an error message indicating that access to URL is forbidden
func (f ForbiddenRequest) Error() string {
	return fmt.Sprintf("Access to %v is forbidden", f.URL)
}
