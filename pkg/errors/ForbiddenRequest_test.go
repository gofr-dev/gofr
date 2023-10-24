package errors

import "testing"

func TestForbiddenRequest_Error(t *testing.T) {
	err := ForbiddenRequest{URL: "dummy/url"}
	expected := "Access to dummy/url is forbidden"

	if err.Error() != expected {
		t.Errorf("Failed: Expected: %v, Got: %v", expected, err)
	}
}
