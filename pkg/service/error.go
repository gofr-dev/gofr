package service

import "fmt"

type FailedRequest struct {
	URL string
	Err error
}

// Error Return's error message about a failed service request, specifically the URL
// of the request and the associated error.
func (f FailedRequest) Error() string {
	return fmt.Sprintf("error in making a service request. URL: %v Error: %v", f.URL, f.Err)
}

type RequestCanceled struct{}

const requestCanceled = "request canceled"

func (r RequestCanceled) Error() string {
	return requestCanceled
}

type ErrServiceDown struct {
	URL string
}

func (e ErrServiceDown) Error() string {
	return fmt.Sprintf("%v is down", e.URL)
}
