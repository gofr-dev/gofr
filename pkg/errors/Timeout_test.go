package errors

import (
	"testing"
)

func TestTimeout_Error(t *testing.T) {
	err := Timeout{URL: "dummy/url"}
	expected := "Request to dummy/url has Timed out!"

	if err.Error() != expected {
		t.Errorf("Failed: Expected: %v, Got: %v", expected, err)
	}
}
