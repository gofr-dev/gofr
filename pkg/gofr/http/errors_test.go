package http

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorEntityNotFound(t *testing.T) {
	fieldName := "id"
	fieldValue := "2"

	err := ErrorEntityNotFound{Name: fieldName, Value: fieldValue}
	expectedMsg := fmt.Sprintf("No entity found with %s : %s", fieldName, fieldValue)

	assert.Equal(t, err.Error(), expectedMsg, "TestErrorEntityNotFound Failed!")
}

func TestErrorEntityNotFound_StatusCode(t *testing.T) {
	err := ErrorEntityNotFound{}
	expectedCode := http.StatusNotFound

	assert.Equal(t, err.StatusCode(), expectedCode, "TestErrorEntityNotFound_StatusCode Failed!")
}

func TestErrorInvalidParam(t *testing.T) {
	tests := []struct {
		desc            string
		params          []string
		expectedMessage string
	}{
		{"no parameter", make([]string, 0), "This request has invalid parameters"},
		{"single parameter", []string{"uuid"}, "Parameter 'uuid' is invalid"},
		{"list of params", []string{"id", "name", "age"}, "Parameters id, name, age are invalid"},
	}

	for i, tc := range tests {
		err := ErrorInvalidParam{Param: tc.params}

		assert.Equal(t, err.Error(), tc.expectedMessage, "TestErrorInvalidParam[%d] : %s Failed!", i,
			tc.desc)
	}
}

func TestInvalidParameter_StatusCode(t *testing.T) {
	err := ErrorInvalidParam{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, err.StatusCode(), expectedCode, "TestErrorInvalidParam_StatusCode Failed!")
}

func TestErrorMissingParam(t *testing.T) {
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
		err := ErrorMissingParam{Param: tc.params}

		assert.Equal(t, err.Error(), tc.expectedMessage, "TestErrorMissingParam[%d] : %s Failed!", i,
			tc.desc)
	}
}

func TestMissingParameter_StatusCode(t *testing.T) {
	err := ErrorMissingParam{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, err.StatusCode(), expectedCode, "TestErrorMissingParam_StatusCode Failed!")
}
