package service

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestResponse_GetHeader(t *testing.T) {
	// Arrange
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	response := &Response{
		headers: headers,
	}

	result := response.GetHeader("Content-Type")
	headerNotFound := response.GetHeader("key")

	assert.Equal(t, "application/json", result)
	assert.Equal(t, "", headerNotFound)
}

func TestResponse_GetHeaderNil(t *testing.T) {
	// Arrange
	response := &Response{}

	result := response.GetHeader("Content-Type")

	assert.Equal(t, "", result)
}
