package gofr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
)

var (
	errTest = errors.New("some error")
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
			err:        errTest,
			statusCode: http.StatusInternalServerError,
		},
	}

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		c := &container.Container{
			Logger: logging.NewLogger(logging.FATAL),
		}

		handler{
			function: func(c *Context) (interface{}, error) {
				return tc.data, tc.err
			},
			container: c,
		}.ServeHTTP(w, r)

		assert.Equalf(t, w.Code, tc.statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
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

func TestHandler_catchAllHandler(t *testing.T) {
	c := Context{
		Context: context.Background(),
	}

	data, err := catchAllHandler(&c)

	assert.Equal(t, data, nil)
	assert.Equal(t, http.ErrMissingFile, err)
}
