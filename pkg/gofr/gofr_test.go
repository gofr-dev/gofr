package gofr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/gofr/testutil"
)

func TestNewCMD(t *testing.T) {
	a := NewCMD()
	// Without args we should get error on stderr.
	outputWithoutArgs := testutil.StderrOutputForFunc(a.Run)
	if outputWithoutArgs != "No Command Found!" {
		t.Errorf("Stderr output mismatch. Got: %s ", outputWithoutArgs)
	}
}

func TestGofr_readConfig(t *testing.T) {
	app := App{}

	app.readConfig()

	if app.Config == nil {
		t.Errorf("config was not read")
	}
}

const helloWorld = "Hello World!"

func TestGofr_ServerRoutes(t *testing.T) {
	type response struct {
		Data interface{} `json:"data"`
	}

	testCases := []struct {
		// Given
		method string
		target string
		// Expectations
		response  string
		headerKey string
		headerVal string
	}{
		{http.MethodGet, "/hello", "Hello World!", "content-type", "application/json"},
		{http.MethodPut, "/hello", "Hello World!", "content-type", "application/json"},
		{http.MethodPost, "/hello", "Hello World!", "content-type", "application/json"},
		{http.MethodGet, "/params?name=Vikash", "Hello Vikash!", "content-type", "application/json"},
	}

	g := New()

	g.GET("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.PUT("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.POST("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.GET("/params", func(c *Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	for _, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.target, http.NoBody)

		r.Header.Set("content-type", "application/json")

		g.httpServer.router.ServeHTTP(w, r)

		var res response
		respBytes, _ := io.ReadAll(w.Body)
		_ = json.Unmarshal(respBytes, &res)

		if res.Data != tc.response {
			t.Errorf("Unexpected response for %s %s. \t expected: %s \t got: %s", tc.method, tc.target, tc.response, res.Data)
		}

		if ctype := w.Header().Get(tc.headerKey); ctype != tc.headerVal {
			t.Errorf("Header mismatch for %s %s. \t expected: %s \t got: %s", tc.method, tc.target, tc.headerVal, ctype)
		}
	}
}

func TestGofr_(t *testing.T) {

}
