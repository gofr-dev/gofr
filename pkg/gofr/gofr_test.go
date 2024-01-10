package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/testutil"
)

const helloWorld = "Hello World!"

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
		{http.MethodGet, "/hello2", "Hello World!", "content-type", "application/json"},
		{http.MethodPut, "/hello", "Hello World!", "content-type", "application/json"},
		{http.MethodPost, "/hello", "Hello World!", "content-type", "application/json"},
		{http.MethodGet, "/params?name=Vikash", "Hello Vikash!", "content-type", "application/json"},
		{http.MethodDelete, "/delete", "Success", "content-type", "application/json"},
	}

	g := New()

	g.GET("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	// using add() func
	g.add(http.MethodGet, "/hello2", func(c *Context) (interface{}, error) {
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

	g.DELETE("/delete", func(c *Context) (interface{}, error) {
		return "Success", nil
	})

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.target, http.NoBody)

		r.Header.Set("content-type", "application/json")

		g.httpServer.router.ServeHTTP(w, r)

		var res response

		respBytes, _ := io.ReadAll(w.Body)
		_ = json.Unmarshal(respBytes, &res)

		assert.Equal(t, res.Data, tc.response, "TEST[%d], Failed.\nUnexpected response for %s %s.", i, tc.method, tc.target)

		assert.Equal(t, w.Header().Get(tc.headerKey), tc.headerVal,
			"TEST[%d], Failed.\nHeader mismatch for %s %s", i, tc.method, tc.target)
	}
}

func TestGofr_ServerRun(t *testing.T) {
	g := New()

	g.GET("/hello", func(c *Context) (interface{}, error) {
		return helloWorld, nil
	})

	go g.Run()
	time.Sleep(1 * time.Second)

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}

	re, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://localhost:"+strconv.Itoa(defaultHTTPPort)+"/hello", http.NoBody)
	resp, err := netClient.Do(re)

	assert.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, resp.StatusCode, http.StatusOK, "TEST Failed.\n")

	resp.Body.Close()
}

func TestGofr_ServerHealthHandlerAddCheck(t *testing.T) {
	var (
		netClient = &http.Client{}
		url       = "http://localhost:8000"
		respn     response.Raw
	)

	g := New()
	// Need to add a route to enable http server
	g.GET("/", nil)
	// Run the server
	go g.Run()
	time.Sleep(1 * time.Second)

	// Send a GET request to the server
	re, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url+"/.well-known/health", http.NoBody)
	resp, err := netClient.Do(re)

	// Assert that connection was successful
	assert.NoError(t, err)

	// make response bytes slice with content-length
	responseBytes := make([]byte, resp.ContentLength)

	// read the response into responseBytes
	_, err = resp.Body.Read(responseBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Errorf("TEST failed: %v", err)

		return
	}

	err = json.Unmarshal(responseBytes, &respn)
	if err != nil {
		t.Errorf("TEST failed %v", err)

		return
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, response.Raw{Data: "OK"}, respn)

	resp.Body.Close()
}

func TestGofr_ServerNotRunningForNoRoutes(t *testing.T) {
	g := New()
	g.httpServer.port = 8001

	go g.Run()

	var (
		netClient = &http.Client{}
		url       = "http://localhost:8001"
	)

	// Send a GET request to the server
	re, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url+"/", http.NoBody)
	resp, err := netClient.Do(re)

	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "connection refused")

	// necessary closing of response body
	if resp != nil {
		resp.Body.Close()
	}
}
