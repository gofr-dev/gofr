package errors

import (
	"testing"
)

func TestEntityNotFound_Error(t *testing.T) {
	err := EntityNotFound{Entity: "brand", ID: "1420"}
	expected := "No 'brand' found for Id: '1420'"

	if err.Error() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, err)
	}
}
