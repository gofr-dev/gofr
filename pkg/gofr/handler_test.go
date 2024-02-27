package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
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
		{"data is nil and error is nil", nil, nil, http.StatusOK},
		{"data is mil, error is not nil", nil, errTest, http.StatusInternalServerError},
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

		assert.Equal(t, w.Code, tc.statusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestHandler_faviconHandler(t *testing.T) {
	c := Context{
		Context: context.Background(),
	}

	d, _ := os.ReadFile("static/favicon.ico")
	data, err := faviconHandler(&c)

	assert.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, data, response.File{
		Content:     d,
		ContentType: "image/x-icon",
	}, "TEST Failed.\n")
}

func TestHandler_catchAllHandler(t *testing.T) {
	c := Context{
		Context: context.Background(),
	}

	data, err := catchAllHandler(&c)

	assert.Equal(t, data, nil, "TEST Failed.\n")

	assert.Equal(t, http.ErrMissingFile, err, "TEST Failed.\n")
}

func TestHandler_livelinessHandler(t *testing.T) {
	resp, err := liveHandler(&Context{})

	assert.Nil(t, err)
	assert.Contains(t, fmt.Sprint(resp), "UP")
}

func TestHandler_healthHandler(t *testing.T) {
	a := New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/alive", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))

	a.AddHTTPService("test-service", server.URL)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "", http.NoBody)

	r := gofrHTTP.NewRequest(req)

	ctx := newContext(nil, r, a.container)

	h, err := healthHandler(ctx)

	assert.Nil(t, err)
	assert.NotNil(t, h)
}
