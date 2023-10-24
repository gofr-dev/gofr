package errors

import (
	"testing"
)

func TestMultipleErrors(t *testing.T) {
	tcs := []struct {
		StatusCode      int
		Errors          []error
		expectedMessage string
	}{
		{404, []error{MissingParam{Param: []string{"ID"}}}, "Parameter ID is required for this request"},
		{400, []error{MissingParam{Param: []string{"Name"}}, InvalidParam{Param: []string{"ID"}}},
			"Parameter Name is required for this request\nIncorrect value for parameter: ID"},
	}

	for i, tc := range tcs {
		err := MultipleErrors{
			StatusCode: tc.StatusCode,
			Errors:     tc.Errors,
		}

		if err.Error() != tc.expectedMessage {
			t.Errorf("Error in testcase[%v]: Expected: %v , Got: %v", i, tc.expectedMessage, err.Error())
		}
	}
}
