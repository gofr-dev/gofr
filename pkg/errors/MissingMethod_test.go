package errors

import (
	"testing"
)

func TestMethodMissing_Error(t *testing.T) {
	err := MethodMissing{Method: "GET", URL: "storeApi"}

	expected := "Method 'GET' for 'storeApi' not defined yet"
	if err.Error() != expected {
		t.Errorf("Failed Expected %v Got %v", expected, err)
	}
}
