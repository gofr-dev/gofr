package errors

import "testing"

func TestEntityAlreadyExists_Error(t *testing.T) {
	err := EntityAlreadyExists{}
	expected := "entity already exists"

	if err.Error() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, err)
	}
}
