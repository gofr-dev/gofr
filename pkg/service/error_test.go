package service

import (
	"testing"

	"gofr.dev/pkg/errors"
)

func TestServiceCall_Error(t *testing.T) {
	tests := []struct {
		URL         string
		Err         error
		expectedErr string
	}{
		{"http://dummy", errors.InvalidParam{Param: []string{"req"}},
			"error in making a service request. URL: http://dummy Error: Incorrect value for parameter: req"},
	}
	for _, tt := range tests {
		s := FailedRequest{tt.URL, tt.Err}
		if got := s.Error(); got != tt.expectedErr {
			t.Errorf("Error() = %v, want %v", got, tt.expectedErr)
		}
	}
}

func TestRequestCanceled_Error(t *testing.T) {
	err := RequestCanceled{}
	expected := requestCanceled

	if err.Error() != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, err)
	}
}

func TestErrServiceDown_Error(t *testing.T) {
	testcases := []struct {
		url string
		msg string
	}{
		{"http://config-service", "http://config-service is down"},
		{"sample-service", "sample-service is down"},
	}

	for i := range testcases {
		err := ErrServiceDown{URL: testcases[i].url}

		if err.Error() != testcases[i].msg {
			t.Errorf("Failed. Got %v\tExpected %v\n", err.Error(), testcases[i].msg)
		}
	}
}
