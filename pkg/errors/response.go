package errors

import "fmt"

// Response is used when a detailed error response needs to be returned
type Response struct {
	StatusCode int         `json:"-"`
	Code       string      `json:"code"`
	Reason     string      `json:"reason"`
	ResourceID string      `json:"resourceId,omitempty"`
	Detail     interface{} `json:"detail,omitempty"`
	Path       string      `json:"path,omitempty"`
	RootCauses []RootCause `json:"rootCauses,omitempty"`
	DateTime   `json:"datetime"`
}

// RootCause denotes the root cause for the error that occurred.
type RootCause map[string]interface{}

// DateTime is a structure to denote the date and time along with the timezone
type DateTime struct {
	Value    string `json:"value"`
	TimeZone string `json:"timezone"`
}

// Error returns a formatted error message with the associated error reason
func (k *Response) Error() string {
	if e, ok := k.Detail.(error); ok {
		return fmt.Sprintf("%v : %v ", k.Reason, e)
	}

	return k.Reason
}

// Error is a generic error of type string
type Error string

// Error returns the error message of type string
func (e Error) Error() string {
	return string(e)
}
