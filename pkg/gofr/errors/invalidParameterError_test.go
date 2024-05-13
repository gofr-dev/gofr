package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidParameter_Error(t *testing.T) {
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
		err := InvalidParamError{Param: tc.params}

		assert.Equal(t, err.Error(), tc.expectedMessage, "TestInvalidParameter_Error[%d] : %s Failed!", i,
			tc.desc)
	}
}

func TestInvalidParameter_StatusCode(t *testing.T) {
	err := InvalidParamError{}
	expectedCode := http.StatusBadRequest

	assert.Equal(t, err.StatusCode(), expectedCode, "TestInvalidParameter_StatusCode Failed!")
}
