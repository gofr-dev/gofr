package gofr

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandler_ServeHTTP(t *testing.T) {
	testCases := []struct {
		desc       string
		data       interface{}
		err        error
		statusCode int
	}{
		{
			desc:       "data is nil and error is nil",
			data:       nil,
			err:        nil,
			statusCode: http.StatusOK,
		},
		{
			desc:       "data is mil, error is not nil",
			data:       nil,
			err:        errors.New("some error"),
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		container := &Container{
			Logger: logging.NewMockLogger(io.Discard),
		}

		handler{
			function: func(c *Context) (interface{}, error) {
				return tc.data, tc.err
			},
			container: container,
		}.ServeHTTP(w, r)

		assert.Equal(t, w.Code, tc.statusCode)
	}
}

func TestHandler_healthHandler(t *testing.T) {
	c := Context{
		Context: context.Background(),
	}

	data, err := healthHandler(&c)

	assert.Equal(t, "OK", data)
	assert.NoError(t, err)
}

func TestHandler_faviconHandler(t *testing.T) {
	c := Context{
		Context: context.Background(),
	}

	d, _ := os.ReadFile("static/favicon.ico")
	data, err := faviconHandler(&c)

	assert.NoError(t, err)
	assert.Equal(t, data, response.File{
		Content:     d,
		ContentType: "image/x-icon",
	})
}
