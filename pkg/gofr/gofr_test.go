package gofr

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

const helloWorld = "Hello World!"
const success = "success"

func TestGofr_ServeHTTP_TextResponse(t *testing.T) {
	testCases := []struct {
		// Given
		method string
		target string
		// Expectations
		response  string
		headerKey string
		headerVal string
	}{
		{http.MethodGet, "/hello", "Hello World!", "content-type", "text/plain"},               // Example 1
		{http.MethodPut, "/hello", "Hello World!", "content-type", "text/plain"},               // Example 1
		{http.MethodPost, "/hello", "Hello World!", "content-type", "text/plain"},              // Example 1
		{http.MethodGet, "/params?name=Vikash", "Hello Vikash!", "content-type", "text/plain"}, // Example 2 with query parameters
	}

	g := New()
	// Added contextInjector middleware
	g.Server.Router.Use(g.Server.contextInjector)
	// Example 1 Handler
	g.GET("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.PUT("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.POST("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	// Example 2 Handler
	g.GET("/params", func(c *Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	for _, tc := range testCases {
		w := httptest.NewRecorder()
		r, _ := request.NewMock(tc.method, tc.target, nil)

		r.Header.Set("content-type", "text/plain")

		g.Server.Router.ServeHTTP(w, r)

		expectedResp := fmt.Sprintf("%v", &types.Response{Data: tc.response})

		if resp := w.Body.String(); resp != expectedResp {
			t.Errorf("Unexpected response for %s %s. \t expected: %s \t got: %s", tc.method, tc.target, expectedResp, resp)
		}

		if ctype := w.Header().Get(tc.headerKey); ctype != tc.headerVal {
			t.Errorf("Header mismatch for %s %s. \t expected: %s \t got: %s", tc.method, tc.target, tc.headerVal, ctype)
		}
	}
}

func TestGofr_StartPanic(t *testing.T) {
	g := New()

	http.DefaultServeMux = new(http.ServeMux)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("Start funcs panics on function call")
			}
		}()
		g.Start()
	}()
	<-time.After(1 * time.Second)
}

func TestGofr_Start(t *testing.T) {
	// only http server should run therefore wrong config location given
	c := config.NewGoDotEnvProvider(log.NewMockLogger(os.Stderr), "../configserror")
	g := NewWithConfig(c)
	g.Server.UseMiddleware(sampleMW1)

	http.DefaultServeMux = new(http.ServeMux)

	go g.Start()
	time.Sleep(3 * time.Second)

	var returned = make(chan bool)

	go func() {
		http.DefaultServeMux = new(http.ServeMux)

		k1 := NewWithConfig(c)
		k1.Start()
		returned <- true
	}()
	time.Sleep(time.Second * 3)

	if !<-returned {
		t.Errorf("Was able to start server on port while server was already running")
	}
}

func TestGofrUseMiddleware(t *testing.T) {
	g := New()
	mws := []Middleware{
		sampleMW1,
		sampleMW2,
	}

	g.Server.UseMiddleware(mws...)

	if len(g.Server.mws) != 2 || !reflect.DeepEqual(g.Server.mws, mws) {
		t.Errorf("FAILED, Expected: %v, Got: %v", mws, g.Server.mws)
	}
}

func TestGofrUseMiddlewarePopulated(t *testing.T) {
	g := New()
	g.Server.mws = []Middleware{
		sampleMW1,
	}

	mws := []Middleware{
		sampleMW2,
	}

	g.Server.UseMiddleware(mws...)

	if len(g.Server.mws) != 2 || reflect.DeepEqual(g.Server.mws, []Middleware{sampleMW1, sampleMW2}) {
		t.Errorf("FAILED, Expected: %v, Got: %v", mws, g.Server.mws)
	}
}

func sampleMW1(h http.Handler) http.Handler {
	return h
}

func sampleMW2(h http.Handler) http.Handler {
	return h
}

func TestGofr_Config(t *testing.T) { // check config is properly set or not?
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../config")
	expected := c.Get("APP_NAME")

	g := New()
	val := g.Config.Get("APP_NAME")

	if !reflect.DeepEqual(expected, val) {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, val)
	}
}

func TestGofr_Patch(t *testing.T) {
	testCases := []struct {
		// Given
		target string
		// Expectations
		expectedCode int
	}{
		{"/patch", 200},
		{"/", 404},
		{"/error", 500},
	}

	// Create a server with PATCH routes
	app := New()
	// Added contextInjector middleware
	app.Server.Router.Use(app.Server.contextInjector)

	app.Server.ValidateHeaders = false

	app.PATCH("/patch", func(c *Context) (interface{}, error) {
		return success, nil
	})

	app.PATCH("/error", func(c *Context) (interface{}, error) {
		return nil, errors.New("sample")
	})

	for _, tc := range testCases {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, tc.target, http.NoBody)

		app.Server.Router.ServeHTTP(rr, r)

		assert.Equal(t, rr.Code, tc.expectedCode)
	}
}

func TestGofr_DELETE(t *testing.T) {
	testCases := []struct {
		desc         string
		target       string
		expResp      string
		expectedCode int
	}{
		{"when the path is /delete", "/delete", "{\"data\":\"success\"}\n", 204},
		{"when path is wrong", "/", "404 page not found\n", 404},
		{"when path is /error", "/error", "{\"errors\":[{\"code\":\"Internal Server Error\",", 500},
	}

	app := New()

	app.Server.Router.Use(app.Server.contextInjector)

	app.Server.ValidateHeaders = false

	app.DELETE("/delete", func(c *Context) (interface{}, error) {
		return success, nil
	})

	app.DELETE("/error", func(c *Context) (interface{}, error) {
		return nil, errors.New("sample")
	})

	for i, tc := range testCases {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodDelete, tc.target, http.NoBody)

		app.Server.Router.ServeHTTP(rr, r)

		assert.Contains(t, rr.Body.String(), tc.expResp, "Test[%d] failed,%v", i, tc.desc)
		assert.Equalf(t, tc.expectedCode, rr.Code, "Test[%d] failed,%v", i, tc.desc)
	}
}

// Test_EnableSwaggerUI to test behavior of EnableSwaggerUI method
func Test_EnableSwaggerUI(t *testing.T) {
	mockConfig := &config.MockConfig{}
	app := NewWithConfig(mockConfig)

	b := new(bytes.Buffer)
	app.Logger = log.NewMockLogger(b)

	app.EnableSwaggerUI()

	routes := fmt.Sprintf("%v", app.Server.Router)

	assert.Contains(t, routes, "/swagger", "Test Failed: routes should contain /swagger route")
	assert.Contains(t, routes, "/swagger/{name}", "Test Failed: routes should contain /swagger/{name} route")
	assert.Contains(t, b.String(), "Usage of EnableSwaggerUI is deprecated", "Test Failed: warning level log should be logged")
}
