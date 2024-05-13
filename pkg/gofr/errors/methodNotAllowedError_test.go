package errors

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMethodNotAllowed_Error(t *testing.T) {
	url := "https://test.com"
	method := http.MethodGet

	err := MethodNotAllowedError{URL: url, Method: method}
	expectedMsg := fmt.Sprintf("Method '%s' is not allowed on URL '%s'", method, url)

	assert.Equal(t, err.Error(), expectedMsg, "TestMethodNotAllowed_Error Failed!")
}

func TestMethodNotAllowed_StatusCode(t *testing.T) {
	err := MethodNotAllowedError{}
	expectedCode := http.StatusMethodNotAllowed

	assert.Equal(t, err.StatusCode(), expectedCode, "TestMethodNotAllowed_StatusCode Failed!")
}
