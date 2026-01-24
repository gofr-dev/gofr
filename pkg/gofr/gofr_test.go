package gofr

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/migration"
	"gofr.dev/pkg/gofr/testutil"
)

const helloWorld = "Hello World!"

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestNewCMD(t *testing.T) {
	a := NewCMD()
	// Without args we should get error on stderr.
	outputWithoutArgs := testutil.StderrOutputForFunc(a.Run)

	assert.Contains(t, outputWithoutArgs, "is not a valid command", "TEST Failed.\n%s", "Stderr output mismatch")
}

func TestGofr_readConfig(t *testing.T) {
	app := App{}

	app.readConfig(false)

	if app.Config == nil {
		t.Errorf("config was not read")
	}
}

func TestGoFr_isPortAvailable(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	tests := []struct {
		name        string
		isAvailable bool
	}{
		{"Port is available", true},
		{"Port is not available", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.isAvailable {
				g := New()

				go g.Run()

				time.Sleep(100 * time.Millisecond)
			}

			isAvailable := isPortAvailable(configs.HTTPPort)
			require.Equal(t, tt.isAvailable, isAvailable)
		})
	}
}

// mockRoundTripper is a mock implementation of http.RoundTripper.
type mockRoundTripper struct {
	lastRequest  *http.Request // Store the last request for assertions
	mockResponse *http.Response
	mockError    error
}

// RoundTrip mocks the HTTP request and stores the request for verification.
func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastRequest = req // Store the request for assertions
	return m.mockResponse, m.mockError
}

func TestPingGoFr(t *testing.T) {
	tests := []struct {
		name        string
		input       bool
		expectedURL string
	}{
		{"Ping Start Server", true, gofrHost + startServerPing},
		{"Ping Shut Server", false, gofrHost + shutServerPing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := &mockRoundTripper{
				mockResponse: &http.Response{
					StatusCode: http.StatusOK,
					Body:       http.NoBody,
				},
				mockError: nil,
			}

			mockClient := &http.Client{Transport: mockTransport}

			_ = testutil.NewServerConfigs(t)

			a := New()

			a.sendTelemetry(mockClient, tt.input)

			assert.NotNil(t, mockTransport.lastRequest, "Request should not be nil")
			assert.Equal(t, tt.expectedURL, mockTransport.lastRequest.URL.String(), "Unexpected request URL")
			assert.Equal(t, http.MethodPost, mockTransport.lastRequest.Method, "Unexpected HTTP method")
		})
	}
}

func TestGofr_ServerRoutes(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	type response struct {
		Data any `json:"data"`
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
		{http.MethodPatch, "/patch", "Success", "content-type", "application/json"},
	}

	g := New()

	g.GET("/hello", func(*Context) (any, error) {
		return helloWorld, nil
	})

	// using add() func
	g.add(http.MethodGet, "/hello2", func(*Context) (any, error) {
		return helloWorld, nil
	})

	g.PUT("/hello", func(*Context) (any, error) {
		return helloWorld, nil
	})

	g.POST("/hello", func(*Context) (any, error) {
		return helloWorld, nil
	})

	g.GET("/params", func(c *Context) (any, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	g.DELETE("/delete", func(*Context) (any, error) {
		return "Success", nil
	})

	g.PATCH("/patch", func(*Context) (any, error) {
		return "Success", nil
	})

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.target, http.NoBody)

		r.Header.Set("Content-Type", "application/json")

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
	configs := testutil.NewServerConfigs(t)

	g := New()

	g.GET("/hello", func(*Context) (any, error) {
		return helloWorld, nil
	})

	go g.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	re, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		"http://localhost:"+fmt.Sprint(configs.HTTPPort)+"/hello", http.NoBody)
	resp, err := netClient.Do(re)

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST Failed.\n")

	resp.Body.Close()
}

func Test_AddHTTPService(t *testing.T) {
	_ = testutil.NewServerConfigs(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))

	g := New()

	g.AddHTTPService("test-service", server.URL)

	resp, _ := g.container.GetHTTPService("test-service").
		Get(t.Context(), "test", nil)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_AddDuplicateHTTPService(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("METRICS_PORT", strconv.Itoa(configs.MetricsPort))
	t.Setenv("HTTP_PORT", strconv.Itoa(configs.HTTPPort))

	logs := testutil.StdoutOutputForFunc(func() {
		a := New()

		a.AddHTTPService("test-service", "http://localhost")
		a.AddHTTPService("test-service", "http://google")
	})

	assert.Contains(t, logs, "Service already registered Name: test-service")
}

func TestApp_Metrics(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	assert.NotNil(t, app.Metrics())
}

func TestApp_MetricsServerDisabled(t *testing.T) {
	// Set METRICS_PORT=0 to disable the metrics server
	t.Setenv("METRICS_PORT", "0")

	logs := testutil.StdoutOutputForFunc(func() {
		app := New()

		// Verify that metricServer is nil when METRICS_PORT=0
		assert.Nil(t, app.metricServer, "metrics server should be nil when METRICS_PORT=0")
	})

	// Verify log message is printed
	assert.Contains(t, logs, "Metrics server is disabled (METRICS_PORT=0)")
}

func TestApp_AddAndGetHTTPService(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	app.AddHTTPService("test-service", "http://test")

	svc := app.container.GetHTTPService("test-service")

	assert.NotNil(t, svc)
}

func TestApp_MigrateInvalidKeys(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		testutil.NewServerConfigs(t)

		app := New()
		app.Migrate(map[int64]migration.Migrate{1: {}})
	})

	assert.Contains(t, logs, "migration run failed! UP not defined for the following keys: [1]")
}

func TestApp_MigratePanicRecovery(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		testutil.NewServerConfigs(t)

		app := New()

		app.container.PubSub = &container.MockPubSub{}

		app.Migrate(map[int64]migration.Migrate{1: {UP: func(_ migration.Datasource) error {
			panic("test panic")
		}}})
	})

	assert.Contains(t, logs, "test panic")
}

func Test_otelErrorHandler(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		h := otelErrorHandler{
			logger: logging.NewLogger(logging.DEBUG),
		}
		h.Handle(testutil.CustomError{ErrorMessage: "OTEL Error override"})
	})

	assert.Contains(t, logs, `"message":"OTEL Error override"`)
	assert.Contains(t, logs, `"level":"ERROR"`)
}

func Test_addRoute(t *testing.T) {
	originalArgs := os.Args // Save the original os.Args

	// Modify os.Args for the duration of this test
	os.Args = []string{"", "log"}

	t.Cleanup(func() { os.Args = originalArgs }) // Restore os.Args after the test

	// Capture the standard output to verify the logs.
	logs := testutil.StdoutOutputForFunc(func() {
		a := NewCMD()

		// Add the "log" sub-command with its handler and description.
		a.SubCommand("log", func(c *Context) (any, error) {
			c.Logger.Info("logging in handler")
			return "handler called", nil
		}, AddDescription("Logs a message"))

		// Run the command-line application.
		a.Run()
	})

	// Verify that the handler was called and the expected log message was output.
	assert.Contains(t, logs, "handler called")
}

func TestEnableBasicAuthWithFunc(t *testing.T) {
	port := testutil.GetFreePort(t)

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	c := container.NewContainer(config.NewMockConfig(nil))

	// Initialize a new App instance
	a := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
			port:   port,
		},
		container: c,
	}

	a.httpServer.router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Println(w, "Hello, world!")
	}))

	a.EnableOAuth(jwksServer.URL, 600)

	server := httptest.NewServer(a.httpServer.router)
	defer server.Close()

	client := server.Client()

	// Create a mock HTTP request
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	// Add a basic authorization header
	req.Header.Add("Authorization", "dXNlcjpwYXNzd29yZA==")

	// Send the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "TestEnableBasicAuthWithFunc Failed!")
}

func encodeBasicAuthorization(t *testing.T, arg string) string {
	t.Helper()

	data := []byte(arg)

	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))

	base64.StdEncoding.Encode(dst, data)

	s := "Basic " + string(dst)

	return s
}

func Test_EnableBasicAuth(t *testing.T) {
	port := testutil.GetFreePort(t)

	mockContainer, _ := container.NewMockContainer(t)

	tests := []struct {
		name               string
		args               []string
		passedCredentials  string
		expectedStatusCode int
	}{
		{
			"No Authorization header passed",
			[]string{"user1", "password1", "user2", "password2"},
			"",
			http.StatusUnauthorized,
		},
		{
			"Even number of arguments",
			[]string{"user1", "password1", "user2", "password2"},
			"user1:password1",
			http.StatusOK,
		},
		{
			"Odd number of arguments with no authorization header passed",
			[]string{"user1", "password1", "user2"},
			"",
			http.StatusOK,
		},
		{
			"Odd number of arguments with wrong authorization header passed",
			[]string{"user1", "password1", "user2"},
			"user1:password2",
			http.StatusOK,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize a new App instance
			a := &App{
				httpServer: &httpServer{
					router: gofrHTTP.NewRouter(),
					port:   port,
				},
				container: mockContainer,
			}

			a.httpServer.router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, "Hello, world!")
			}))

			a.EnableBasicAuth(tt.args...)

			server := httptest.NewServer(a.httpServer.router)
			defer server.Close()

			client := server.Client()

			// Create a mock HTTP request
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
			require.NoError(t, err)

			// Add a basic authorization header
			req.Header.Add("Authorization", encodeBasicAuthorization(t, tt.passedCredentials))

			// Send the HTTP request
			resp, err := client.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}

func Test_EnableBasicAuthWithValidator(t *testing.T) {
	port := testutil.GetFreePort(t)

	mockContainer, _ := container.NewMockContainer(t)

	tests := []struct {
		name               string
		passedCredentials  string
		expectedStatusCode int
	}{
		{
			"No Authorization header passed",
			"",
			http.StatusUnauthorized,
		},
		{
			"Correct Authorization",
			"user:password",
			http.StatusOK,
		},
		{
			"Wrong Authorization header passed",
			"user2:password2",
			http.StatusUnauthorized,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize a new App instance
			a := &App{
				httpServer: &httpServer{
					router: gofrHTTP.NewRouter(),
					port:   port,
				},
				container: mockContainer,
			}

			validateFunc := func(_ *container.Container, username string, password string) bool {
				return username == "user" && password == "password"
			}

			a.EnableBasicAuthWithValidator(validateFunc)

			a.httpServer.router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, "Hello, world!")
			}))

			server := httptest.NewServer(a.httpServer.router)
			defer server.Close()

			client := server.Client()

			// Create a mock HTTP request
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
			require.NoError(t, err)

			// Add a basic authorization header
			req.Header.Add("Authorization", encodeBasicAuthorization(t, tt.passedCredentials))

			// Send the HTTP request
			resp, err := client.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}

func Test_AddRESTHandlers(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	type user struct {
		ID   int
		Name string
	}

	var invalidObject int

	tests := []struct {
		desc  string
		input any
		err   error
	}{
		{"success case", &user{}, nil},
		{"invalid object", &invalidObject, errInvalidObject},
		{"invalid object", user{}, fmt.Errorf("failed to register routes for 'user' struct, %w", errNonPointerObject)},
		{"invalid object", nil, errObjectIsNil},
	}

	for i, tc := range tests {
		err := app.AddRESTHandlers(tc.input)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_initTracer(t *testing.T) {
	createMockConfig := func(traceExporter, url, authKey string) config.Config {
		return config.NewMockConfig(map[string]string{
			"TRACE_EXPORTER":  traceExporter,
			"TRACER_URL":      url,
			"TRACER_AUTH_KEY": authKey,
		})
	}
	mockConfig1 := createMockConfig("zipkin", "http://localhost:2005/api/v2/spans", "")

	mockConfig2 := createMockConfig("zipkin", "http://localhost:2005/api/v2/spans", "valid-token")

	mockConfig3 := createMockConfig("jaeger", "localhost:4317", "")

	mockConfig4 := createMockConfig("jaeger", "localhost:4317", "valid-token")

	mockConfig5 := createMockConfig("otlp", "localhost:4317", "")

	mockConfig6 := createMockConfig("otlp", "localhost:4317", "valid-token")

	mockConfig7 := createMockConfig("gofr", "", "")

	tests := []struct {
		desc               string
		config             config.Config
		expectedLogMessage string
	}{
		{"tracing disabled", config.NewMockConfig(nil), "tracing is disabled"},
		{"zipkin exporter", mockConfig1, "Exporting traces to zipkin at http://localhost:2005/api/v2/spans"},
		{"zipkin exporter with authkey", mockConfig2, "Exporting traces to zipkin at http://localhost:2005/api/v2/spans"},
		{"jaeger exporter", mockConfig3, "Exporting traces to jaeger at localhost:4317"},
		{"jaeger exporter with auth", mockConfig4, "Exporting traces to jaeger at localhost:4317"},
		{"otlp exporter", mockConfig5, "Exporting traces to otlp at localhost:4317"},
		{"otlp exporter with authKey", mockConfig6, "Exporting traces to otlp at localhost:4317"},
		{"gofr exporter with default url", mockConfig7, "Exporting traces to GoFr at https://tracer-api.gofr.dev/api/spans"},
	}

	for i, tc := range tests {
		logMessage := testutil.StdoutOutputForFunc(func() {
			mockContainer, _ := container.NewMockContainer(t)

			a := App{
				Config:    tc.config,
				container: mockContainer,
			}
			a.initTracer()
		})
		assert.Contains(t, logMessage, tc.expectedLogMessage, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_initTracer_invalidConfig(t *testing.T) {
	createMockConfig := func(traceExporter, url, authKey string) config.Config {
		return config.NewMockConfig(map[string]string{
			"TRACE_EXPORTER":  traceExporter,
			"TRACER_URL":      url,
			"TRACER_AUTH_KEY": authKey,
		})
	}
	mockConfig1 := createMockConfig("abc", "https://tracer-service.dev", "")
	mockConfig2 := createMockConfig("", "https://tracer-service.dev", "")
	mockConfig3 := createMockConfig("otlp", "", "")

	testErr := []struct {
		desc               string
		config             config.Config
		expectedLogMessage string
	}{
		{"unsupported trace_exporter", mockConfig1, "unsupported TRACE_EXPORTER: abc"},
		{"missing trace_exporter", mockConfig2, "missing TRACE_EXPORTER config, should be provided with TRACER_URL to enable tracing"},
		{"miss tracer_url ", mockConfig3,
			"missing TRACER_URL config, should be provided with TRACE_EXPORTER to enable tracing"},
	}

	for i, tc := range testErr {
		logMessage := testutil.StderrOutputForFunc(func() {
			mockContainer, _ := container.NewMockContainer(t)

			a := App{
				Config:    tc.config,
				container: mockContainer,
			}
			a.initTracer()
		})

		assert.Contains(t, logMessage, tc.expectedLogMessage, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_UseMiddleware(t *testing.T) {
	port := testutil.GetFreePort(t)

	testMiddleware := func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-Middleware", "applied")
			inner.ServeHTTP(w, r)
		})
	}

	c := container.NewContainer(config.NewMockConfig(nil))

	app := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
			port:   port,
		},
		container: c,
		Config: config.NewMockConfig(map[string]string{
			"REQUEST_TIMEOUT":       "5",
			"SHUTDOWN_GRACE_PERIOD": "1s",
		}),
	}

	app.UseMiddleware(testMiddleware)

	app.GET("/test", func(*Context) (any, error) {
		return "success", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		fmt.Sprintf("http://localhost:%d", port)+"/test", http.NoBody)

	resp, err := netClient.Do(req)
	if err != nil {
		t.Errorf("error while making HTTP request in Test_UseMiddleware. err : %v", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Test_UseMiddleware Failed! Expected Status 200 Got : %v", resp.StatusCode)

	// checking if the testMiddleware has added the required header in the response properly.
	testHeaderValue := resp.Header.Get("X-Test-Middleware")
	assert.Equal(t, "applied", testHeaderValue, "Test_UseMiddleware Failed! header value mismatch.")
}

// Test the UseMiddlewareWithContainer function.
func TestUseMiddlewareWithContainer(t *testing.T) {
	port := testutil.GetFreePort(t)

	// Initialize the mock container
	mockContainer := container.NewContainer(config.NewMockConfig(nil))

	// Create a simple handler to test middleware functionality
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, world!"))
	})

	// Middleware to modify response and test container access
	middleware := func(c *container.Container, handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Ensure the container is passed correctly (for this test, we are just logging)
			assert.NotNil(t, c, "Container should not be nil in the middleware")

			// Continue with the handler execution
			handler.ServeHTTP(w, r)
		})
	}

	// Create a new App with a mock server
	app := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
			port:   port,
		},
		container: mockContainer,
		Config:    config.NewMockConfig(map[string]string{"REQUEST_TIMEOUT": "5"}),
	}

	// Use the middleware with the container
	app.UseMiddlewareWithContainer(middleware)

	// Register the handler to a route for testing
	app.httpServer.router.Handle("/test", handler)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	// Create a test response recorder
	rr := httptest.NewRecorder()

	// Call the handler with the request and recorder
	app.httpServer.router.ServeHTTP(rr, req)

	// Assert the status code and response body
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Hello, world!", rr.Body.String())
}

func Test_APIKeyAuthMiddleware(t *testing.T) {
	port := testutil.GetFreePort(t)

	c, _ := container.NewMockContainer(t)

	app := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
			port:   port,
		},
		container: c,
		Config:    config.NewMockConfig(map[string]string{"REQUEST_TIMEOUT": "5"}),
	}

	apiKeys := []string{"test-key"}
	validateFunc := func(_ *container.Container, apiKey string) bool {
		return apiKey == "test-key"
	}

	// Registering APIKey middleware with and without custom validator
	app.EnableAPIKeyAuth(apiKeys...)
	app.EnableAPIKeyAuthWithValidator(validateFunc)

	app.GET("/test", func(_ *Context) (any, error) {
		return "success", nil
	})

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		fmt.Sprintf("http://localhost:%d", port)+"/test", http.NoBody)
	req.Header.Set("X-Api-Key", "test-key")

	// Send the request and check for successful response
	resp, err := netClient.Do(req)
	if err != nil {
		t.Errorf("error while making HTTP request in Test_APIKeyAuthMiddleware. err: %v", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Test_APIKeyAuthMiddleware Failed!")
}

func Test_SwaggerEndpoints(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	// Create the openapi.json file within the static directory
	openAPIFilePath := filepath.Join("static", OpenAPIJSON)

	openAPIContent := []byte(`{"swagger": "2.0", "info": {"version": "1.0.0", "title": "Sample API"}}`)
	if err := os.WriteFile(openAPIFilePath, openAPIContent, 0600); err != nil {
		t.Errorf("Failed to create openapi.json file: %v", err)
		return
	}

	// Defer removal of swagger file from the static directory
	defer func() {
		if err := os.RemoveAll("static/openapi.json"); err != nil {
			t.Errorf("Failed to remove swagger file from static directory: %v", err)
		}
	}()

	app := New()
	app.httpRegistered = true
	app.httpServer.port = configs.HTTPPort

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	re, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		configs.HTTPHost+"/.well-known/swagger", http.NoBody)
	resp, err := netClient.Do(re)

	defer func() {
		err = resp.Body.Close()
		if err != nil {
			t.Errorf("error closing response body: %v", err)
		}
	}()

	require.NoError(t, err, "Expected error to be nil, got : %v", err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
}

func Test_AddCronJob_Fail(t *testing.T) {
	a := App{container: &container.Container{}}
	stderr := testutil.StderrOutputForFunc(func() {
		a.container.Logger = logging.NewLogger(logging.ERROR)

		a.AddCronJob("* * * *", "test-job", func(ctx *Context) {
			ctx.Logger.Info("test-job-fail")
		})
	})

	assert.Contains(t, stderr, "error adding cron job")
	assert.NotContains(t, stderr, "test-job-fail")
}

func Test_AddCronJob_Success(t *testing.T) {
	pass := false
	a := App{
		container: &container.Container{},
	}

	a.AddCronJob("* * * * *", "test-job", func(ctx *Context) {
		ctx.Logger.Info("test-job-success")
	})

	assert.Len(t, a.cron.jobs, 1)

	for _, j := range a.cron.jobs {
		if j.name == "test-job" {
			pass = true
			break
		}
	}

	assert.Truef(t, pass, "unable to add cron job to cron table")
}

func setupTestEnvironment(t *testing.T) (host string, htmlContent []byte) {
	t.Helper()
	configs := testutil.NewServerConfigs(t)

	// Generating some files for testing
	htmlContent = []byte("<html><head><title>Test Static File</title></head><body><p>Testing Static File</p></body></html>")

	createPublicDirectory(t, defaultPublicStaticDir, htmlContent)

	createPublicDirectory(t, "testdir", htmlContent)

	app := New()

	app.AddStaticFiles("gofrTest", "testdir")

	app.httpRegistered = true
	app.httpServer.port = configs.HTTPPort

	go app.Run()

	time.Sleep(100 * time.Millisecond)

	host = configs.HTTPHost

	return host, htmlContent
}

func TestStaticHandler(t *testing.T) {
	const indexHTML = "indexTest.html"

	host, htmlContent := setupTestEnvironment(t)

	defer os.Remove("static/indexTest.html")
	defer os.RemoveAll("testdir")

	tests := []struct {
		desc                       string
		method                     string
		path                       string
		statusCode                 int
		expectedBody               string
		expectedBodyLength         int
		expectedResponseHeaderType string
	}{
		{
			desc: "check file content index.html", method: http.MethodGet, path: "/" + defaultPublicStaticDir + "/" + indexHTML,
			statusCode: http.StatusOK, expectedBodyLength: len(htmlContent),
			expectedResponseHeaderType: "text/html; charset=utf-8", expectedBody: string(htmlContent),
		},
		{
			desc: "check public endpoint", method: http.MethodGet,
			path: "/" + defaultPublicStaticDir, statusCode: http.StatusNotFound,
		},
		{
			desc: "check file content index.html in custom dir", method: http.MethodGet, path: "/" + "gofrTest" + "/" + indexHTML,
			statusCode: http.StatusOK, expectedBodyLength: len(htmlContent),
			expectedResponseHeaderType: "text/html; charset=utf-8", expectedBody: string(htmlContent),
		},
		{
			desc: "check public endpoint in custom dir", method: http.MethodGet, path: "/" + "gofrTest",
			statusCode: http.StatusNotFound,
		},
	}

	for i, tc := range tests {
		request, err := http.NewRequestWithContext(t.Context(), tc.method, host+tc.path, http.NoBody)
		if err != nil {
			t.Fatalf("TEST[%d], Failed to create request, error: %s", i, err)
		}

		request.Header.Set("Content-Type", "application/json")

		client := http.Client{}

		resp, err := client.Do(request)
		if err != nil {
			t.Fatalf("TEST[%d], Request failed, error: %s", i, err)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("TEST[%d], Failed to read response body, error: %s", i, err)
		}

		body := string(bodyBytes)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed with Status Body.\n%s", i, tc.desc)

		if tc.expectedBody != "" {
			assert.Contains(t, body, tc.expectedBody, "TEST[%d], Failed with Expected Body.\n%s", i, tc.desc)
		}

		if tc.expectedBodyLength != 0 {
			contentLength := resp.Header.Get("Content-Length")
			assert.Equal(t, strconv.Itoa(tc.expectedBodyLength), contentLength, "TEST[%d], Failed at Content-Length.\n%s", i, tc.desc)
		}

		if tc.expectedResponseHeaderType != "" {
			assert.Equal(t,
				tc.expectedResponseHeaderType,
				resp.Header.Get("Content-Type"),
				"TEST[%d], Failed at Expected Content-Type.\n%s", i, tc.desc)
		}

		resp.Body.Close()
	}
}

func TestStaticHandlerInvalidFilePath(t *testing.T) {
	// Generating some files for testing
	logs := testutil.StderrOutputForFunc(func() {
		testutil.NewServerConfigs(t)

		app := New()

		app.AddStaticFiles("gofrTest", ".//,.!@#$%^&")
	})

	assert.Contains(t, logs, "no such file or directory")
	assert.Contains(t, logs, "error in registering '/gofrTest' static endpoint")
}

func createPublicDirectory(t *testing.T, defaultPublicStaticDir string, htmlContent []byte) {
	t.Helper()

	const indexHTML = "indexTest.html"

	directory := "./" + defaultPublicStaticDir
	if _, err := os.Stat(directory); err != nil {
		if err = os.Mkdir("./"+defaultPublicStaticDir, os.ModePerm); err != nil {
			t.Fatalf("Couldn't create a "+defaultPublicStaticDir+" directory, error: %s", err)
		}
	}

	file, err := os.Create(filepath.Join(directory, indexHTML))
	if err != nil {
		t.Fatalf("Couldn't create %s file", indexHTML)
	}

	_, err = file.Write(htmlContent)
	if err != nil {
		t.Fatalf("Couldn't write to %s file", indexHTML)
	}

	file.Close()
}

func Test_Shutdown(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		testutil.NewServerConfigs(t)

		g := New()

		g.GET("/hello", func(*Context) (any, error) {
			return helloWorld, nil
		})

		go g.Run()

		time.Sleep(10 * time.Millisecond)

		err := g.Shutdown(t.Context())

		require.NoError(t, err, "Test_Shutdown Failed!")
	})

	assert.Contains(t, logs, "Application shutdown complete", "Test_Shutdown Failed!")
}

func TestApp_SubscriberInitialize(t *testing.T) {
	t.Run("subscriber is initialized", func(t *testing.T) {
		testutil.NewServerConfigs(t)

		app := New()

		mockContainer := container.Container{
			Logger: logging.NewLogger(logging.ERROR),
			PubSub: mockSubscriber{},
		}

		app.container = &mockContainer

		app.Subscribe("Hello", func(*Context) error {
			// this is a test subscriber
			return nil
		})

		_, ok := app.subscriptionManager.subscriptions["Hello"]

		assert.True(t, ok)
	})

	t.Run("subscriber is not initialized", func(t *testing.T) {
		testutil.NewServerConfigs(t)

		app := New()
		app.Subscribe("Hello", func(*Context) error {
			// this is a test subscriber
			return nil
		})

		_, ok := app.subscriptionManager.subscriptions["Hello"]

		assert.False(t, ok)
	})
}

func TestApp_Subscribe(t *testing.T) {
	t.Run("topic is empty", func(t *testing.T) {
		testutil.NewServerConfigs(t)

		app := New()

		mockContainer := container.Container{
			Logger: logging.NewLogger(logging.ERROR),
			PubSub: mockSubscriber{},
		}

		app.container = &mockContainer

		app.Subscribe("", func(*Context) error { return nil })

		_, ok := app.subscriptionManager.subscriptions[""]

		assert.False(t, ok)
	})

	t.Run("handler is nil", func(t *testing.T) {
		testutil.NewServerConfigs(t)

		app := New()

		mockContainer := container.Container{
			Logger: logging.NewLogger(logging.ERROR),
			PubSub: mockSubscriber{},
		}

		app.container = &mockContainer

		app.Subscribe("Hello", nil)

		_, ok := app.subscriptionManager.subscriptions["Hello"]

		assert.False(t, ok)
	})
}

// Define static error for testing.
var errHookFailed = errors.New("hook failed")

func TestApp_OnStart(t *testing.T) {
	// Test case 1: Hook executes successfully
	t.Run("success", func(t *testing.T) {
		var hookCalled bool

		app := New()

		app.OnStart(func(_ *Context) error {
			hookCalled = true
			return nil
		})

		err := app.runOnStartHooks(t.Context())

		require.NoError(t, err, "Expected no error from runOnStartHooks")
		assert.True(t, hookCalled, "Expected the OnStart hook to be called")
	})

	// Test case 2: Hook returns an error
	t.Run("error", func(t *testing.T) {
		app := New()

		app.OnStart(func(_ *Context) error {
			return errHookFailed
		})

		err := app.runOnStartHooks(t.Context())

		require.ErrorIs(t, err, errHookFailed, "Expected an error from runOnStartHooks")
	})

	// Test case 4: Verify panic recovery
	t.Run("panic recovery", func(t *testing.T) {
		app := New()

		app.OnStart(func(_ *Context) error {
			panic("test panic")
		})

		err := app.runOnStartHooks(t.Context())

		require.Error(t, err, "Expected error from panicked hook")
		assert.Contains(t, err.Error(), "panicked", "Expected error message to mention panic")
	})
}
func TestUnifiedAuthenticationRegistration(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	t.Setenv("HTTP_PORT", strconv.Itoa(testutil.GetFreePort(t)))

	app := New()

	// Enable various auth methods
	app.EnableBasicAuth("user", "pass")
	app.EnableAPIKeyAuth("key1")
	app.EnableOAuth("http://jwks", 3600)

	// Verify HTTP middleware count (approximate check)
	// We can't easily inspect the router's middleware slice directly without reflection or exposing it,
	// but we can check if the grpcServer has interceptors added.
	assert.GreaterOrEqual(t, len(app.grpcServer.interceptors), 2, "gRPC unary interceptors should be registered")
	assert.GreaterOrEqual(t, len(app.grpcServer.streamInterceptors), 2, "gRPC stream interceptors should be registered")
}
