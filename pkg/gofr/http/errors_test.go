package http

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestErrorEntityNotFound(t *testing.T) {
	fieldName := "id"
	fieldValue := "2"

	err := ErrorEntityNotFound{Name: fieldName, Value: fieldValue}
	expectedMsg := fmt.Sprintf("No entity found with %s: %s", fieldName, fieldValue)

	require.ErrorContainsf(t, err, expectedMsg, "TEST Failed.\n")
}

func TestErrorEntityNotFound_StatusCode(t *testing.T) {
	err := ErrorEntityNotFound{}
	expectedCode := http.StatusNotFound

	assert.Equal(t, expectedCode, err.StatusCode(), "TEST Failed.\n")
}

func TestErrorEntityAlreadyExist(t *testing.T) {
	err := ErrorEntityAlreadyExist{}

	require.ErrorContains(t, err, alreadyExistsMessage, "TEST Failed.\n")
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

		require.ErrorContainsf(t, err, tc.expectedMsg, "TEST[%d], Failed.\n%s", i, tc.desc)
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

		require.ErrorContainsf(t, err, tc.expectedMsg, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestMissingParameter_StatusCode(t *testing.T) {
	err := ErrorMissingParam{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, expectedCode, err.StatusCode(), "TEST Failed.\n")
}

func TestErrorInvalidRoute(t *testing.T) {
	err := ErrorInvalidRoute{}

	require.ErrorContainsf(t, err, "route not registered", "TEST Failed.\n")

	assert.Equal(t, http.StatusNotFound, err.StatusCode(), "TEST Failed.\n")
}

func Test_ErrorRequestTimeout(t *testing.T) {
	err := ErrorRequestTimeout{}

	require.ErrorContainsf(t, err, "request timed out", "TEST Failed.\n")

	assert.Equal(t, http.StatusRequestTimeout, err.StatusCode(), "TEST Failed.\n")
}

func Test_ErrorErrorPanicRecovery(t *testing.T) {
	err := ErrorPanicRecovery{}

	require.ErrorContainsf(t, err, http.StatusText(http.StatusInternalServerError), "TEST Failed.\n")

	assert.Equal(t, http.StatusInternalServerError, err.StatusCode(), "TEST Failed.\n")
}

func Test_ServiceUnavailable(t *testing.T) {
	code503 := http.StatusServiceUnavailable
	testCases := []struct {
		message      string
		dependency   string
		errorMessage string
		statusCode   int
		logLevel     logging.Level
	}{
		{"", "", http.StatusText(code503), code503, logging.ERROR},
		{"Connection Error", "", http.StatusText(code503), code503, logging.ERROR},
		{"", "DB", http.StatusText(code503), code503, logging.ERROR},
		{"Connection Error", "DB", "Service unavailable due to error: Connection Error from dependency DB", code503, logging.ERROR},
	}

	for _, tc := range testCases {
		err := ErrorServiceUnavailable{
			Dependency:   tc.dependency,
			ErrorMessage: tc.message,
		}
		assert.Equal(t, tc.statusCode, err.StatusCode())
		assert.Equal(t, tc.errorMessage, err.Error())
		assert.Equal(t, tc.logLevel, err.LogLevel())
	}
}
