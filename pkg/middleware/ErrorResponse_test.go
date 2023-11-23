package middleware

import (
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/errors"
)

//nolint:gocognit // all conditions are required for the test
func Test_fetchErrResponseWithCode(t *testing.T) {
	zone, _ := time.Now().Zone()

	tcs := []struct {
		statusCode int
		reason     string
		code       string
	}{
		{http.StatusUnauthorized, "UnAuthorised", "401"},
		{http.StatusInternalServerError, "Internal Server Error", "PANIC"},
	}

	for _, tc := range tcs {
		err := FetchErrResponseWithCode(tc.statusCode, tc.reason, tc.code)
		if err == nil {
			t.Errorf("Expected not nil, got nil")
			continue
		}

		if err.StatusCode != tc.statusCode {
			t.Errorf("Expected status code: %v, got: %v", tc.statusCode, err.StatusCode)
		}

		if len(err.Errors) != 1 {
			t.Errorf("Expected Errors size 1, got %d", len(err.Errors))
		} else if errorResponse, ok := err.Errors[0].(*errors.Response); ok {
			if errorResponse.Code != tc.code {
				t.Errorf("Expected Code %v, got %v", tc.code, errorResponse.Code)
			}
			if errorResponse.Reason != tc.reason {
				t.Errorf("Expected Reason %v, got %v", tc.reason, errorResponse.Reason)
			}
			if errorResponse.DateTime.Value != time.Now().Format(time.RFC3339) {
				t.Errorf("Expected TimeValue %v, got %v", time.Now().Format(time.RFC3339), errorResponse.DateTime.Value)
			}
			if errorResponse.DateTime.TimeZone != zone {
				t.Errorf("Expected TimeZone %v, got %v", zone, errorResponse.DateTime.TimeZone)
			}
		} else {
			t.Errorf("Expected error.Errors[0] to be of type *errors.Response, got: %T", err.Errors)
		}
	}
}
