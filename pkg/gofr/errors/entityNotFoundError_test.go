package errors

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntityNotFoundError_Error(t *testing.T) {
	fieldName := "id"
	fieldValue := "2"

	err := EntityNotFoundError{fieldName: fieldName, fieldValue: fieldValue}
	expectedMsg := fmt.Sprintf("No entity found with %s : %s", fieldName, fieldValue)

	assert.Equal(t, err.Error(), expectedMsg, "TestEntityNotFoundError_Error Failed!")
}

func TestEntityNotFoundError_StatusCode(t *testing.T) {
	err := EntityNotFoundError{}
	expectedCode := http.StatusNotFound

	assert.Equal(t, err.StatusCode(), expectedCode, "TestEntityNotFoundError_StatusCode Failed!")
}
