package errors

import "fmt"

// MethodMissing is used when the requested method is not present for the called endpoint (used for method not allowed)
type MethodMissing struct {
	Method string
	URL    string
}

// Error returns an error message indicating that the method for a given URL is not defined
func (m MethodMissing) Error() string {
	return fmt.Sprintf("Method '%s' for '%s' not defined yet", m.Method, m.URL)
}
