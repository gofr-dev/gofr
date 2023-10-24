package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetHeader(t *testing.T) {
	tcs := []struct {
		response Response
		expected string
	}{
		{Response{headers: http.Header{"Content-Length": {"20"}}}, "20"},
		{Response{headers: nil}, ""},
	}
	for i, tc := range tcs {
		assert.Equal(t, tc.expected, tc.response.GetHeader("Content-Length"), i)
	}
}
