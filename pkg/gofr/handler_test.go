package gofr

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errTest = errors.New("some error")
)

func TestHandler_ServeHTTP(t *testing.T) {
	testCases := []struct {
		desc       string
		method     string
		data       any
		err        error
		statusCode int
		body       string
	}{
		{"method is get, data is nil and error is nil", http.MethodGet, nil, nil, http.StatusOK,
			`{}`},
		{"method is get, data is mil, error is not nil", http.MethodGet, nil, errTest, http.StatusInternalServerError,
			`{"error":{"message":"some error"}}`},
		{"method is get, data is mil, error is http error", http.MethodGet, nil, gofrHTTP.ErrorEntityNotFound{}, http.StatusNotFound,
			`{"error":{"message":"No entity found with : "}}`},
		{"method is post, data is nil and error is nil", http.MethodPost, "Created", nil, http.StatusCreated,
			`{"data":"Created"}`},
		{"method is delete, data is nil and error is nil", http.MethodDelete, nil, nil, http.StatusNoContent,
			`{}`},
	}

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, "/", http.NoBody)
		c := &container.Container{
			Logger: logging.NewLogger(logging.FATAL),
		}

		handler{
			function: func(*Context) (any, error) {
				return tc.data, tc.err
			},
			container: c,
		}.ServeHTTP(w, r)

		assert.Containsf(t, w.Body.String(), tc.body, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.statusCode, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestHandler_ServeHTTP_Timeout(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	h := handler{requestTimeout: 100 * time.Millisecond}

	h.container = &container.Container{Logger: logging.NewLogger(logging.FATAL)}
	h.function = func(*Context) (any, error) {
		time.Sleep(200 * time.Millisecond)

		return "hey", nil
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusRequestTimeout, w.Code, "TestHandler_ServeHTTP_Timeout Failed")

	assert.Contains(t, w.Body.String(), "request timed out", "TestHandler_ServeHTTP_Timeout Failed")
}

func TestHandler_ServeHTTP_Panic(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	h := handler{}

	h.container = &container.Container{Logger: logging.NewLogger(logging.FATAL)}
	h.function = func(*Context) (any, error) {
		panic("runtime panic")
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "TestHandler_ServeHTTP_Panic Failed")

	assert.Contains(t, w.Body.String(), http.StatusText(http.StatusInternalServerError), "TestHandler_ServeHTTP_Panic Failed")
}

func TestHandler_ServeHTTP_WithHeaders(t *testing.T) {
	testCases := []struct {
		desc       string
		method     string
		data       any
		headers    map[string]string
		err        error
		statusCode int
		body       string
	}{
		{
			desc:   "Response with headers, method is GET, no error",
			method: http.MethodGet,
			data: response.Response{
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"Content-Type":    "application/json",
				},
				Data: map[string]string{
					"message": "Hello, World!",
				},
			},
			headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"Content-Type":    "application/json",
			},
			statusCode: http.StatusOK,
			body:       `{"message":"Hello, World!"}`,
		},
		{
			desc:       "No headers, method is GET, data is simple string, no error",
			method:     http.MethodGet,
			data:       "simple string",
			statusCode: http.StatusOK,
			body:       `"simple string"`,
		},
	}

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, "/", http.NoBody)
		c := &container.Container{
			Logger: logging.NewLogger(logging.FATAL),
		}

		handler{
			function: func(*Context) (any, error) {
				return tc.data, tc.err
			},
			container: c,
		}.ServeHTTP(w, r)

		assert.Containsf(t, w.Body.String(), tc.body, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)

		for key, expectedValue := range tc.headers {
			assert.Equal(t, expectedValue, w.Header().Get(key), "TEST[%d], Failed. Header mismatch: %s", i, key)
		}
	}
}

func TestHandler_faviconHandlerError(t *testing.T) {
	c := Context{
		Context: t.Context(),
	}

	d, _ := os.ReadFile("static/favicon.ico")

	// renaming the file to produce the error case and rename it back to original after completion of test.
	_, err := os.Stat("static/favicon.ico")
	if err != nil {
		t.Errorf("favicon.ico file not found in static directory")
		return
	}

	err = os.Rename("static/favicon.ico", "static/newFavicon.ico")
	if err != nil {
		t.Errorf("error in renaming favicon.ico!")
	}

	defer func() {
		err = os.Rename("static/newFavicon.ico", "static/favicon.ico")
		if err != nil {
			t.Errorf("error in renaming file back to favicon.ico")
		}
	}()

	data, err := faviconHandler(&c)

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, response.File{
		Content:     d,
		ContentType: "image/x-icon",
	}, data, "TEST Failed.\n")
}

func TestHandler_faviconHandler(t *testing.T) {
	c := Context{
		Context: t.Context(),
	}

	d, _ := os.ReadFile("static/favicon.ico")
	data, err := faviconHandler(&c)

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, response.File{
		Content:     d,
		ContentType: "image/x-icon",
	}, data, "TEST Failed.\n")
}

func TestHandler_catchAllHandler(t *testing.T) {
	c := Context{
		Context: t.Context(),
	}

	data, err := catchAllHandler(&c)

	assert.Nil(t, data, "TEST Failed.\n")

	assert.Equal(t, gofrHTTP.ErrorInvalidRoute{}, err, "TEST Failed.\n")
}

func TestHandler_livelinessHandler(t *testing.T) {
	resp, err := liveHandler(&Context{})

	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(resp), "UP")
}

func TestHandler_healthHandler(t *testing.T) {
	testutil.NewServerConfigs(t)

	a := New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/alive", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))

	a.AddHTTPService("test-service", server.URL)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)

	r := gofrHTTP.NewRequest(req)

	ctx := newContext(nil, r, a.container)

	h, err := healthHandler(ctx)

	require.NoError(t, err)
	assert.NotNil(t, h)
}
