package errors

import "testing"

func TestMissingTemplate_Error(t *testing.T) {
	testcases := []struct {
		fileLocation  string
		fileName      string
		expectedError string
	}{
		{
			"", "default.html", "Template default.html does not exist at location: ",
		},
		{
			"fake/directory/", "default.html", "Template default.html does not exist at location: fake/directory/",
		},
		{
			"", "", "Filename not provided",
		},
	}
	for i, testcase := range testcases {
		customError := MissingTemplate{
			FileLocation: testcase.fileLocation,
			FileName:     testcase.fileName,
		}
		if customError.Error() != testcase.expectedError {
			t.Errorf("Error in testcase[%v]: Expected: %v , Got: %v", i, testcase.expectedError, customError.Error())
		}
	}
}
