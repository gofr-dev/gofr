package errors

import (
	"testing"
)

func TestInvalidParam_ErrorMessage(t *testing.T) {
	testCases := []struct {
		error        InvalidParam
		errorMessage string
	}{
		{
			error: InvalidParam{Param: []string{
				"organizationId",
				"userId",
			}},
			errorMessage: "Incorrect value for parameters: organizationId, userId",
		},
		{
			error: InvalidParam{Param: []string{
				"organizationId",
			}},
			errorMessage: "Incorrect value for parameter: organizationId",
		},
		{
			error:        InvalidParam{Param: nil},
			errorMessage: "This request has invalid parameters",
		},
	}

	for _, tc := range testCases {
		if tc.error.Error() != tc.errorMessage {
			t.Errorf("FAILED, Expected: %v, Got: %v", tc.errorMessage, tc.error.Error())
		}
	}
}
