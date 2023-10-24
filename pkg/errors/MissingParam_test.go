package errors

import "testing"

func TestMissingParam_ErrorMessage(t *testing.T) {
	testCases := []struct {
		error        MissingParam
		errorMessage string
	}{
		{
			error: MissingParam{Param: []string{
				"organizationId",
				"userId",
			}},
			errorMessage: "Parameters organizationId, userId are required for this request",
		},
		{
			error: MissingParam{Param: []string{
				"organizationId",
			}},
			errorMessage: "Parameter organizationId is required for this request",
		},
		{
			error:        MissingParam{Param: nil},
			errorMessage: "This request is missing parameters",
		},
	}

	for _, tc := range testCases {
		if tc.error.Error() != tc.errorMessage {
			t.Errorf("FAILED, Expected: %v, Got: %v", tc.errorMessage, tc.error.Error())
		}
	}
}
