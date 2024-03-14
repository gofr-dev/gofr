package errors

import "testing"

func TestEntityAlreadyExists_Error(t *testing.T) {
	err := EntityAlreadyExists{}
	if err.Error() != errMessage {
		t.Errorf("FAILED, Expected: %v, Got: %v", errMessage, err)
	}
}
