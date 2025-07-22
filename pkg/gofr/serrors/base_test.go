package serrors

import "testing"

// compile time interface check
func TestErrorImplementation(t *testing.T) {
	var _ IError = &Error{}
}
