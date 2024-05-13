package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMissingParameter_Error(t *testing.T) {
	tests := []struct {
		desc            string
		params          []string
		expectedMessage string
	}{
		{"no parameter", make([]string, 0), "This request is missing parameters"},
		{"single parameter", []string{"uuid"},
			"1 parameter(s) uuid are missing for this request"},
		{"list of params", []string{"id", "name", "age"},
			"3 parameter(s) id, name, age are missing for this request"},
	}

	for i, tc := range tests {
		err := MissingParamError{Param: tc.params}

		assert.Equal(t, err.Error(), tc.expectedMessage, "TestMissingParameter_Error[%d] : %s Failed!", i,
			tc.desc)
	}
}

func TestMissingParameter_StatusCode(t *testing.T) {
	err := MissingParamError{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, err.StatusCode(), expectedCode, "TestMissingParameter_StatusCode Failed!")
}
