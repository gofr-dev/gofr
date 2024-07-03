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
	expectedMsg := fmt.Sprintf("No entity found with %s: %s", fieldName, fieldValue)

	assert.Equal(t, expectedMsg, err.Error(), "TEST Failed.\n")
}

func TestErrorEntityNotFound_StatusCode(t *testing.T) {
	err := ErrorEntityNotFound{}
	expectedCode := http.StatusNotFound

	assert.Equal(t, expectedCode, err.StatusCode(), "TEST Failed.\n")
}

func TestErrorEntityAlreadyExist(t *testing.T) {
	err := ErrorEntityAlreadyExist{}

	assert.Equal(t, alreadyExistsMessage, err.Error(), "TEST Failed.\n")
}

func TestErrorEntityAlreadyExist_StatusCode(t *testing.T) {
	err := ErrorEntityAlreadyExist{}
	expectedCode := http.StatusConflict

	assert.Equal(t, expectedCode, err.StatusCode(), "TEST Failed.\n")
}

func TestErrorInvalidParam(t *testing.T) {
	tests := []struct {
		desc        string
		params      []string
		expectedMsg string
	}{
		{"no parameter", make([]string, 0), "'0' invalid parameter(s): "},
		{"single parameter", []string{"uuid"}, "'1' invalid parameter(s): uuid"},
		{"list of params", []string{"id", "name", "age"}, "'3' invalid parameter(s): id, name, age"},
	}

	for i, tc := range tests {
		err := ErrorInvalidParam{Params: tc.params}

		assert.Equal(t, tc.expectedMsg, err.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestInvalidParameter_StatusCode(t *testing.T) {
	err := ErrorInvalidParam{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, expectedCode, err.StatusCode(), "TestErrorInvalidParam_StatusCode Failed!")
}

func TestErrorMissingParam(t *testing.T) {
	tests := []struct {
		desc        string
		params      []string
		expectedMsg string
	}{
		{"no parameter", make([]string, 0), "'0' missing parameter(s): "},
		{"single parameter", []string{"uuid"}, "'1' missing parameter(s): uuid"},
		{"list of params", []string{"id", "name", "age"}, "'3' missing parameter(s): id, name, age"},
	}

	for i, tc := range tests {
		err := ErrorMissingParam{Params: tc.params}

		assert.Equal(t, tc.expectedMsg, err.Error(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestMissingParameter_StatusCode(t *testing.T) {
	err := ErrorMissingParam{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, expectedCode, err.StatusCode(), "TEST Failed.\n")
}

func TestErrorInvalidRoute(t *testing.T) {
	err := ErrorInvalidRoute{}

	assert.Equal(t, "route not registered", err.Error(), "TEST Failed.\n")

	assert.Equal(t, http.StatusNotFound, err.StatusCode(), "TEST Failed.\n")
}

func Test_ErrorRequestTimeout(t *testing.T) {
	err := ErrorRequestTimeout{}

	assert.Equal(t, "request timed out", err.Error(), "TEST Failed.\n")

	assert.Equal(t, http.StatusRequestTimeout, err.StatusCode(), "TEST Failed.\n")
}

func Test_ErrorErrorPanicRecovery(t *testing.T) {
	err := ErrorPanicRecovery{}

	assert.Equal(t, http.StatusText(http.StatusInternalServerError), err.Error(), "TEST Failed.\n")

	assert.Equal(t, http.StatusInternalServerError, err.StatusCode(), "TEST Failed.\n")
}
