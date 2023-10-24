package errors

import (
	"testing"
)

func Test_HealthCheckFailed_Error(t *testing.T) {
	testcases := []struct {
		err      error
		expError string
	}{
		{HealthCheckFailed{Dependency: "dep", Reason: "dep not initialized"}, "Health check failed for dep Reason: dep not initialized"},
		{HealthCheckFailed{Dependency: "dep", Err: DB{}}, "Health check failed for dep Error: DB Error"},
		{HealthCheckFailed{Dependency: "dep", Reason: "dep not initialized", Err: DB{}},
			"Health check failed for dep Reason: dep not initialized Error: DB Error"},
		{HealthCheckFailed{Dependency: "dep"}, "Health check failed for dep"},
	}
	for i, tc := range testcases {
		if tc.err.Error() != tc.expError {
			t.Errorf("TEST [%v] FAILED, Expected: %v, Got: %v", i, tc.expError, tc.err)
		}
	}
}
