package gofr

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"gofr.dev/pkg"

	"github.com/stretchr/testify/assert"
)

func TestRouteLog(t *testing.T) {
	g := New()

	g.GET("/", func(c *Context) (interface{}, error) { return helloWorld, nil })
	g.GET("/hello-world", func(c *Context) (interface{}, error) { return helloWorld, nil })
	g.GET("/hello-world/", func(c *Context) (interface{}, error) { return helloWorld, nil })
	g.POST("/hello-world", func(c *Context) (interface{}, error) { return helloWorld, nil })
	g.POST("/hello-world/", func(c *Context) (interface{}, error) { return helloWorld, nil })
	g.POST("/hello", func(c *Context) (interface{}, error) { return helloWorld, nil })
	g.POST("/hello/", func(c *Context) (interface{}, error) { return helloWorld, nil })

	// should not be returned from logRoutes() as method is invalid
	g.Server.Router.Route("", "/failed", func(c *Context) (interface{}, error) { return helloWorld, nil })

	// should not be returned from logRoutes() as path is invalid
	g.POST("$$$$$", func(c *Context) (interface{}, error) { return helloWorld, nil })

	expected := "GET / HEAD / GET /hello-world HEAD /hello-world POST /hello-world POST /hello "

	got := fmt.Sprintf("%s"+"", g.Server.Router)

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
}

// TestRouter_WellKnownEndpoint test Router for well-known endpoints
func TestRouter_WellKnownEndpoint(t *testing.T) {
	testcases := []struct {
		desc        string
		route       string
		prefix      string
		expectedLog string
	}{
		{"case when route is /hello-world and  prefix is empty", "/hello-world", "",
			"GET /hello-world HEAD /hello-world GET /.well-known/health-check " + "HEAD /.well-known/health-check GET " +
				"/.well-known/heartbeat HEAD /.well-known/heartbeat "},
		{"case when route is /hello-world and prefix is /api", "/hello-world", "/api",
			"GET /api/hello-world HEAD /api/hello-world GET /.well-known/health-check HEAD" +
				" /.well-known/health-check GET /.well-known/heartbeat HEAD /.well-known/heartbeat "},
		{"case when route is hello-world and prefix is api/", "hello-world", "api/", ""},
		{"case when route is hello-world/ and prefix is api/", "hello-world/", "api/", ""},
		{"case when route is /hello-world/ and prefix is empty", "/hello-world/", "", "GET /hello-world HEAD /hello-world " +
			"GET /.well-known/health-check HEAD /.well-known/health-check GET /.well-known/heartbeat HEAD /.well-known/heartbeat "},
		{"case when route is /hello-world and when prefix is api ", "/hello-world", "api", ""},
		{"case when route is /hello-world and prefix is api/", "/hello-world", "api/", ""},
		{"case when route is /hello-world when prefix is /api/", "/hello-world", "/api/", "GET /api//hello-world HEAD" +
			" /api//hello-world GET /.well-known/health-check HEAD /.well-known/health-check GET" +
			" /.well-known/heartbeat HEAD /.well-known/heartbeat "},
	}
	for i, tc := range testcases {
		g := New()

		g.Server.Router.Prefix(tc.prefix)
		g.GET(tc.route, func(c *Context) (interface{}, error) { return helloWorld, nil })

		go g.Start()

		time.Sleep(3 * time.Second)

		assert.Equal(t, tc.expectedLog, fmt.Sprintf("%s", g.Server.Router), "Test Failed[%d]:%v", i, tc.desc)
	}
}

// Test_isWellKnownEndPoint is taken to test the behavior of isWellKnownEndPoint function
func Test_isWellKnownEndPoint(t *testing.T) {
	testcase := []struct {
		desc    string
		path    string
		expResp bool
	}{
		{"success case when health check path is given", pkg.PathHealthCheck, true},
		{"success case when heart beat path is given", pkg.PathHeartBeat, true},
		{"success case when openAPI path is given", pkg.PathOpenAPI, true},
		{"success case when swagger path is given", pkg.PathSwagger, true},
		{"success case when swagger with pathparam path is given", pkg.PathSwaggerWithPathParam, true},
		{"failure case as path is incomplete", "/.well-known/health", false},
	}
	for i, tc := range testcase {
		resp := isWellKnownEndPoint(tc.path)
		assert.Equal(t, tc.expResp, resp, "Test [%d]case failed,%v", i, tc.desc)
	}
}
