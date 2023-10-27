package gofr

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware/oauth"
)

func TestContextInjector(t *testing.T) {
	s := &server{}
	s.contextPool.New = func() interface{} {
		return NewContext(nil, nil, New())
	}

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/", nil)

	w := httptest.NewRecorder()

	handler := s.contextInjector(innerHandler)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, s.contextPool.Get())
}

func Test_getMWVars(t *testing.T) {
	conf := &config.MockConfig{Data: map[string]string{
		"VALIDATE_HEADERS":       "TRUE",
		"Access-Control-Max-Age": "2000",
	}}

	expectedResult := map[string]string{
		"VALIDATE_HEADERS":       "TRUE",
		"Access-Control-Max-Age": "2000",
		"LOG_OMIT_HEADERS":       "",
	}

	result := getMWVars(conf)

	assert.Equal(t, expectedResult, result, "TEST FAILED.")
}

// check whether the default value for ValidateHeaders is set to false or not
func TestHeaderValidation(t *testing.T) {
	// start a server using Gofr
	app := New()

	if app.Server.ValidateHeaders {
		t.Errorf("header validation set true")
	}
}

//func TestServer_Done(t *testing.T) {
//	// start a server using Gofr
//	g := New()
//	g.Server.HTTP.Port = 8080
//
//	go g.Start()
//	time.Sleep(time.Second * 3)
//
//	serverUP := false
//
//	// check if server is up
//	for i := 0; i < 2; i++ {
//		resp, _ := http.Get("http://localhost:8080/.well-known/heartbeat")
//		if resp.StatusCode == http.StatusOK {
//			serverUP = true
//			_ = resp.Body.Close()
//
//			break
//		}
//
//		time.Sleep(time.Second)
//	}
//
//	if !serverUP {
//		t.Errorf("server not up")
//	}
//
//	// stop the server
//	g.Server.Done()
//
//	serverUP = true
//
//	// check if the server is down
//	for i := 0; i < 3; i++ {
//		//nolint:bodyclose // there is no response here hence body cannot be closed.
//		_, err := http.Get("http://localhost:8080/.well-known/heartbeat")
//		// expecting an error since server is down
//		if err != nil {
//			serverUP = false
//
//			break
//		}
//
//		time.Sleep(time.Second)
//	}
//
//	if serverUP {
//		t.Errorf("server down failed")
//	}
//}
//
//// This tests if a server can be started again after being stopped.
//func TestServer_Done2(t *testing.T) {
//	TestServer_Done(t)
//	TestServer_Done(t)
//}

// Test_AllRouteLog will test logging of all routes of the server along with methods
func Test_AllRouteLog(t *testing.T) {
	g := New()
	g.Server.HTTP.Port = 8080

	b := new(bytes.Buffer)
	g.Logger = log.NewMockLogger(b)

	go g.Start()
	time.Sleep(time.Second * 2)
	assert.Contains(t, b.String(), "GET /.well-known/health-check HEAD /.well-known/health-check ")
	assert.Contains(t, b.String(), "GET /.well-known/heartbeat HEAD /.well-known/heartbeat ")
	assert.NotContains(t, b.String(), "\"NotFoundHandler\":null,\"MethodNotAllowedHandler\":null,\"KeepContext\":false")
}

func Test_APIFilePresent(t *testing.T) {
	g := New()
	g.Server.HTTP.Port = 8080

	wd, _ := os.Getwd()
	_ = os.Mkdir(wd+"/api", 0777)

	defer os.RemoveAll(wd + "/api/")

	_, _ = os.Create(wd + "/api/openapi.json")

	b := new(bytes.Buffer)
	g.Logger = log.NewMockLogger(b)

	go g.Start()
	time.Sleep(time.Second * 2)
	assert.Contains(t, b.String(), "GET /.well-known/openapi.json HEAD /.well-known/openapi.json ")
	assert.Contains(t, b.String(), "GET /.well-known/swagger HEAD /.well-known/swagger ")
	assert.Contains(t, b.String(), "GET /.well-known/swagger/{name} HEAD /.well-known/swagger/{name}")
}

func Test_APIFileNotPresent(t *testing.T) {
	g := New()
	g.Server.HTTP.Port = 8080

	b := new(bytes.Buffer)
	g.Logger = log.NewMockLogger(b)

	go g.Start()
	time.Sleep(time.Second * 2)
	assert.NotContains(t, b.String(), "GET /.well-known/openapi.json HEAD /.well-known/openapi.json ")
	assert.NotContains(t, b.String(), "GET /.well-known/swagger HEAD /.well-known/swagger ")
	assert.NotContains(t, b.String(), "GET /.well-known/swagger/{name} HEAD /.well-known/swagger/{name}")
}

// TestRouter_CatchAllRoute tests the CatchAllRoute for the requests to not registered endpoints or invalid routes.
func TestRouter_CatchAllRoute(t *testing.T) {
	app := New()

	app.Server.ValidateHeaders = false

	app.GET("/dummy", func(ctx *Context) (interface{}, error) {
		return nil, nil
	})

	go app.Start()
	time.Sleep(time.Second * 2)

	tests := []struct {
		desc       string
		endpoint   string
		method     string
		statusCode int
	}{
		{"invalid route", "/dummy1", http.MethodGet, http.StatusNotFound},
		{"substring route", "/dumm", http.MethodPost, http.StatusNotFound},
		{"valid route", "/dummy", http.MethodGet, http.StatusOK},
		{"invalid method", "/dummy", http.MethodDelete, http.StatusMethodNotAllowed},
	}

	for i, test := range tests {
		req, _ := http.NewRequest(test.method, "http://localhost:8000"+test.endpoint, http.NoBody)
		client := http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] %v\nerror while making request, %v", i, test.desc, err)
			continue
		}

		if resp.StatusCode != test.statusCode {
			t.Errorf("TEST[%v] %v\n expected %v, got %v", i, test.desc, test.statusCode, resp.StatusCode)
		}

		_ = resp.Body.Close()
	}
}

func Test_setupAuth(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	g := &Gofr{Logger: logger}

	c1 := &config.MockConfig{Data: map[string]string{
		"JWKS_ENDPOINT":        "/abc",
		"OAUTH_CACHE_VALIDITY": "2000",
		"LDAP_ADDR":            "xyz",
	}}

	c2 := &config.MockConfig{Data: map[string]string{
		"JWKS_ENDPOINT":        "/abc",
		"OAUTH_CACHE_VALIDITY": "2000",
	}}

	tests := []struct {
		desc       string
		logMessage string
		config     *config.MockConfig
	}{
		{"using middleware", "OAuth middleware not enabled due to LDAP_ADDR env variable set", c1},
		{"without LDAP_ADDR", "", c2},
	}

	for i, tc := range tests {
		s := NewServer(tc.config, g)
		s.setupAuth(tc.config, g)

		if !strings.Contains(b.String(), tc.logMessage) {
			t.Errorf("Test case [%d] failed\n%s", i, tc.desc)
		}
	}
}

func TestGetOAuthOptions(t *testing.T) {
	cfg1 := &config.MockConfig{Data: map[string]string{
		"JWKS_ENDPOINT":        "",
		"OAUTH_CACHE_VALIDITY": "2000",
	}}

	cfg2 := &config.MockConfig{Data: map[string]string{
		"JWKS_ENDPOINT":        "/abc",
		"OAUTH_CACHE_VALIDITY": "aaaab",
	}}

	cfg3 := &config.MockConfig{Data: map[string]string{
		"JWKS_ENDPOINT":        "/abc",
		"OAUTH_CACHE_VALIDITY": "8000",
	}}

	tests := []struct {
		desc    string
		config  *config.MockConfig
		options oauth.Options
		expOut  bool
	}{
		{"invalid JWKPath", cfg1, oauth.Options{}, false},
		{"invalid OAUTH_CACHE_VALIDITY", cfg2, oauth.Options{ValidityFrequency: 1800, JWKPath: "/abc"}, true},
		{"valid configs", cfg3, oauth.Options{ValidityFrequency: 8000, JWKPath: "/abc"}, true},
	}

	for i, tc := range tests {
		output, ok := getOAuthOptions(tc.config)

		assert.Equal(t, tc.options, output, "Test case [%d] failed\n%s", i, tc.desc)

		assert.Equal(t, tc.expOut, ok, "Test case [%d] failed\n%s", i, tc.desc)
	}
}

func TestIsEndpointWithPathParam(t *testing.T) {
	givenEndpoints := map[string]bool{
		"/users/{id}":       true,
		"/posts/{postId}":   true,
		"/products/{sku}":   true,
		"/categories/{id}":  true,
		"/orders":           true,
		"/login":            true,
		"/logout":           true,
		"/calc/{name}/{id}": true,
	}

	tests := []struct {
		actualEndpoint  string
		expectedPattern string
		expectedResult  bool
	}{
		{"/users/123", "/users/{id}", true},
		{"/posts/abc", "/posts/{postId}", true},
		{"/products/xyz", "/products/{sku}", true},
		{"/categories/456", "/categories/{id}", true},
		{"/calc/abc/45", "/calc/{name}/{id}", true},
		{"/orders", "/orders", true},
		{"/login", "/login", true},
		{"/logout", "/logout", true},
		{"/categories", "", false},
		{"/orders/123", "", false},
		{"/login/123", "", false},
		{"/logout/123", "", false},
		{"calc/abc", "", false},
	}

	for i, tc := range tests {
		pattern, result := isEndpointWithPathParam(givenEndpoints, tc.actualEndpoint)

		if pattern != tc.expectedPattern || result != tc.expectedResult {
			t.Errorf("Test[%d] Failed,For endpoint %s, expected pattern: %s, expected result: %v, but got pattern: %s, result: %v", i,
				tc.actualEndpoint, tc.expectedPattern, tc.expectedResult, pattern, result)
		}
	}
}

func TestFilterValidRoutes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{"valid routes", "/route1 /route2 /route3 /.well-known/route4",
			map[string]bool{"/route1": true, "/route2": true, "/route3": true}},
		{"no valid routes", "/.well-known/route4", map[string]bool{}},
		{"empty input", "", map[string]bool{}},
		{"mixed routes", "/route1 /route2 /.well-known/route4 /route3",
			map[string]bool{"/route1": true, "/route2": true, "/route3": true}},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterValidRoutes(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("TEST[%d] Failed,Expected %v, but got %v", i, tc.expected, result)
			}
		})
	}
}

func TestExemptPathParam(t *testing.T) {
	configs := config.MockConfig{}
	app := NewWithConfig(&configs)
	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	sampleHandler := func(ctx *Context) (interface{}, error) {
		return "hello", nil
	}

	app.GET("/users/{id}", sampleHandler)

	handler := app.Server.removePathParamValueFromTraces()(&MockHandler{})
	handler.ServeHTTP(w, req)

	newPath := req.Context().Value("path")
	if newPath != "/users/{id}" {
		t.Errorf("Expected context value '/users/{id}', but got '%s'", newPath)
	}
}

type MockHandler struct{}

// ServeHTTP is used for testing if the request context has traceId
func (r *MockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("test"))
}
