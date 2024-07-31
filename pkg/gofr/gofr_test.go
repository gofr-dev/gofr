package gofr

import (
	"context"
	"encoding/json"
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

func TestNewCMD(t *testing.T) {
	a := NewCMD()
	// Without args we should get error on stderr.
	outputWithoutArgs := testutil.StderrOutputForFunc(a.Run)

	assert.Equal(t, "No Command Found!\n", outputWithoutArgs, "TEST Failed.\n%s", "Stderr output mismatch")
}

func TestGofr_readConfig(t *testing.T) {
	app := App{}

	app.readConfig(false)

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
		{http.MethodPatch, "/patch", "Success", "content-type", "application/json"},
	}

	g := New()

	g.GET("/hello", func(*Context) (interface{}, error) {
		return helloWorld, nil
	})

	// using add() func
	g.add(http.MethodGet, "/hello2", func(*Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.PUT("/hello", func(*Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.POST("/hello", func(*Context) (interface{}, error) {
		return helloWorld, nil
	})

	g.GET("/params", func(c *Context) (interface{}, error) {
		return fmt.Sprintf("Hello %s!", c.Param("name")), nil
	})

	g.DELETE("/delete", func(*Context) (interface{}, error) {
		return "Success", nil
	})

	g.PATCH("/patch", func(*Context) (interface{}, error) {
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

	g.GET("/hello", func(*Context) (interface{}, error) {
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

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST Failed.\n")

	resp.Body.Close()
}

func Test_AddHTTPService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))

	g := New()

	g.AddHTTPService("test-service", server.URL)

	resp, _ := g.container.GetHTTPService("test-service").
		Get(context.Background(), "test", nil)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_AddDuplicateHTTPService(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")

	logs := testutil.StdoutOutputForFunc(func() {
		a := New()

		a.AddHTTPService("test-service", "http://localhost")
		a.AddHTTPService("test-service", "http://google")
	})

	assert.Contains(t, logs, "Service already registered Name: test-service")
}

func TestApp_Metrics(t *testing.T) {
	app := New()

	assert.NotNil(t, app.Metrics())
}

func TestApp_AddAndGetHTTPService(t *testing.T) {
	app := New()

	app.AddHTTPService("test-service", "http://test")

	svc := app.container.GetHTTPService("test-service")

	assert.NotNil(t, svc)
}

func TestApp_MigrateInvalidKeys(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		app := New()
		app.Migrate(map[int64]migration.Migrate{1: {}})
	})

	assert.Contains(t, logs, "migration run failed! UP not defined for the following keys: [1]")
}

func TestApp_MigratePanicRecovery(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
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
		h := otelErrorHandler{logging.NewLogger(logging.DEBUG)}
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
		a.SubCommand("log", func(c *Context) (interface{}, error) {
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
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	c := container.NewContainer(config.NewMockConfig(nil))

	// Initialize a new App instance
	a := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
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

func Test_AddRESTHandlers(t *testing.T) {
	app := New()

	type user struct {
		ID   int
		Name string
	}

	var invalidObject int

	tests := []struct {
		desc  string
		input interface{}
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
		{"zipkin exporter", mockConfig1, "Exporting traces to zipkin at http://localhost:2005/api/v2/spans"},
		{"zipkin exporter with authkey", mockConfig2, "Exporting traces to zipkin at http://localhost:2005/api/v2/spans"},
		{"jaeger exporter", mockConfig3, "Exporting traces to jaeger at localhost:4317"},
		{"jaeger exporter with auth", mockConfig4, "Exporting traces to jaeger at localhost:4317"},
		{"otlp exporter", mockConfig5, "Exporting traces to otlp at localhost:4317"},
		{"otlp exporter with authKey", mockConfig6, "Exporting traces to otlp at localhost:4317"},
		{"gofr exporter with default url", mockConfig7, "Exporting traces to GoFr at https://tracer.gofr.dev"},
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
			port:   8001,
		},
		container: c,
		Config:    config.NewMockConfig(map[string]string{"REQUEST_TIMEOUT": "5"}),
	}

	app.UseMiddleware(testMiddleware)

	app.GET("/test", func(*Context) (interface{}, error) {
		return "success", nil
	})

	go app.Run()
	time.Sleep(1 * time.Second)

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://localhost:8001"+"/test", http.NoBody)

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

func Test_APIKeyAuthMiddleware(t *testing.T) {
	c, _ := container.NewMockContainer(t)

	app := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
			port:   8001,
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

	app.GET("/test", func(_ *Context) (interface{}, error) {
		return "success", nil
	})

	go app.Run()
	time.Sleep(1 * time.Second)

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://localhost:8001/test", http.NoBody)
	req.Header.Set("X-API-Key", "test-key")

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
	app.httpServer.port = 8002

	go app.Run()
	time.Sleep(1 * time.Second)

	var netClient = &http.Client{
		Timeout: time.Second * 5,
	}

	re, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://localhost:8002"+"/.well-known/swagger", http.NoBody)
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

func TestStaticHandler(t *testing.T) {
	const indexHTML = "indexTest.html"

	// Generating some files for testing
	htmlContent := []byte("<html><head><title>Test Static File</title></head><body><p>Testing Static File</p></body></html>")

	createPublicDirectory(t, defaultPublicStaticDir, htmlContent)

	defer os.Remove("static/indexTest.html")

	createPublicDirectory(t, "testdir", htmlContent)

	defer os.RemoveAll("testdir")

	app := New()

	app.AddStaticFiles("gofrTest", "./testdir")

	app.httpRegistered = true
	app.httpServer.port = 8022

	go app.Run()
	time.Sleep(1 * time.Second)

	host := "http://localhost:8022"

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
		request, err := http.NewRequestWithContext(context.Background(), tc.method, host+tc.path, http.NoBody)
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
		g := New()

		g.GET("/hello", func(*Context) (interface{}, error) {
			return helloWorld, nil
		})

		go g.Run()
		time.Sleep(10 * time.Millisecond)

		err := g.Shutdown(context.Background())

		require.NoError(t, err, "Test_Shutdown Failed!")
	})

	assert.Contains(t, logs, "Application shutdown complete", "Test_Shutdown Failed!")
}
