package gofr

import (
	"bytes"
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
	if outputWithoutArgs != "No Command Found!" {
		t.Errorf("Stderr output mismatch. Got: %s ", outputWithoutArgs)
	}
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

	assert.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, resp.StatusCode, http.StatusOK, "TEST Failed.\n")

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
	logs := testutil.StdoutOutputForFunc(func() {
		a := NewCMD()

		a.SubCommand("log", func(c *Context) (interface{}, error) {
			c.Logger.Info("logging in handler")

			return "handler called", nil
		})

		a.Run()
	})

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
	}

	for i, tc := range tests {
		err := app.AddRESTHandlers(tc.input)

		assert.Equal(t, tc.err, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_initTracer(t *testing.T) {
	mockConfig1 := config.NewMockConfig(map[string]string{
		"TRACE_EXPORTER": "zipkin",
		"TRACER_HOST":    "localhost",
		"TRACER_PORT":    "2005",
	})

	mockConfig2 := config.NewMockConfig(map[string]string{
		"TRACE_EXPORTER": "jaeger",
		"TRACER_HOST":    "localhost",
		"TRACER_PORT":    "2005",
	})

	mockConfig3 := config.NewMockConfig(map[string]string{
		"TRACE_EXPORTER": "gofr",
	})

	tests := []struct {
		desc               string
		config             config.Config
		expectedLogMessage string
	}{
		{"zipkin exporter", mockConfig1, "Exporting traces to zipkin."},
		{"jaeger exporter", mockConfig2, "Exporting traces to jaeger."},
		{"gofr exporter", mockConfig3, "Exporting traces to GoFr at https://tracer.gofr.dev"},
	}

	for _, tc := range tests {
		logMessage := testutil.StdoutOutputForFunc(func() {
			mockContainer, _ := container.NewMockContainer(t)

			a := App{
				Config:    tc.config,
				container: mockContainer,
			}

			a.initTracer()
		})

		assert.Contains(t, logMessage, tc.expectedLogMessage)
	}
}

func Test_initTracer_invalidConfig(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"TRACE_EXPORTER": "abc",
		"TRACER_HOST":    "localhost",
		"TRACER_PORT":    "2005",
	})

	errLogMessage := testutil.StderrOutputForFunc(func() {
		mockContainer, _ := container.NewMockContainer(t)

		a := App{
			Config:    mockConfig,
			container: mockContainer,
		}

		a.initTracer()
	})

	assert.Contains(t, errLogMessage, "unsupported trace exporter.")
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

	assert.Nil(t, err, "Expected error to be nil, got : %v", err)
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

	assert.Equal(t, len(a.cron.jobs), 1)

	for _, j := range a.cron.jobs {
		if j.name == "test-job" {
			pass = true
			break
		}
	}

	assert.Truef(t, pass, "unable to add cron job to cron table")
}

func TestStaticHandler(t *testing.T) {
	// Application Generation
	app := New()

	//Generate a public directory endpoint
	directory := "./public"

	if _, err := os.Stat(directory); err != nil {
		err := os.Mkdir("./public", 0755)
		if err != nil {
			t.Errorf("Couldn't create a public directory, error: %s", err)
		}
	}

	app.AddStaticFiles("/public", directory)

	// Generate some files for testing
	svgFileContent1 := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" width=\"731.67\" height=\"511.12\" viewBox=\"0 0 731.67 511.12\"><path d=\"m0,509.7c0,.66.53,1.19,1.19,1.19h729.29c.66,0,1.19-.53,1.19-1.19s-.53-1.19-1.19-1.19H1.19c-.66,0-1.19.53-1.19,1.19Z\" fill=\"#3f3d58\"/><polygon points=\"440.61 79.12 466.22 87.54 466.22 50.67 442.98 50.67 440.61 79.12\" fill=\"#f8a8ab\"/><circle cx=\"463.05\" cy=\"35.35\" r=\"25.52\" fill=\"#f8a8ab\"/><path d=\"m456.55,37.35l3.52,4.27,6.36-11.14s8.12.42,8.12-5.61,7.45-6.2,7.45-6.2c0,0,10.55-18.42-11.3-13.57,0,0-15.16-10.38-22.69-1.51,0,0-23.11,11.64-16.5,31.9l10.99,20.89,2.49-4.73s-1.51-19.85,11.56-14.32v.02Z\" fill=\"#2f2e43\"/><rect x=\"432.93\" y=\"461.78\" width=\"20.94\" height=\"29.71\" fill=\"#f8a8ab\"/><path d=\"m451.55,508.51c-3.58.32-21.5,1.74-22.4-2.37-.82-3.77.39-7.71.56-8.25,1.72-17.14,2.36-17.33,2.75-17.44.61-.18,2.39.67,5.28,2.53l.18.12.04.21c.05.27,1.33,6.56,7.4,5.59,4.16-.66,5.51-1.58,5.94-2.03-.35-.16-.79-.44-1.1-.92-.45-.7-.53-1.6-.23-2.68.78-2.85,3.12-7.06,3.22-7.23l.27-.48,23.8,16.06,14.7,4.2c1.11.32,2,1.11,2.45,2.17h0c.62,1.48.24,3.2-.96,4.28-2.67,2.4-7.97,6.51-13.54,7.02-1.48.14-3.44.19-5.64.19-9.19,0-22.61-.95-22.71-.97h0Z\" fill=\"#2f2e43\"/><path d=\"m480.61,205.64l-54.93-2.81s-8.42,31.92,2.22,65.18l1.28,200.29h31.04l29.26-206.61-8.87-56.05h0Z\" fill=\"#2f2e43\"/><path d=\"m471.35,72.03l-30.15-16s-32.49,47.48-28,73.2c4.5,25.72,12.48,73.6,12.48,73.6l66.51,2.81-11.61-94.29-9.23-39.32s0,0,0,0Z\" fill=\"#e2e3e4\"/><rect x=\"447.83\" y=\"461.78\" width=\"20.94\" height=\"29.71\" fill=\"#f8a8ab\"/><path d=\"m466.45,508.51c-3.58.32-21.5,1.74-22.4-2.37-.82-3.77.39-7.71.56-8.25,1.72-17.14,2.36-17.33,2.75-17.44.61-.18,2.39.67,5.28,2.53l.18.12.04.21c.05.27,1.33,6.56,7.4,5.59,4.16-.66,5.51-1.58,5.94-2.03-.35-.16-.79-.44-1.1-.92-.45-.7-.53-1.6-.23-2.68.78-2.85,3.12-7.06,3.22-7.23l.27-.48,23.8,16.06,14.7,4.2c1.11.32,2,1.11,2.45,2.17h0c.62,1.48.24,3.2-.96,4.28-2.67,2.4-7.97,6.51-13.54,7.02-1.48.14-3.44.19-5.64.19-9.19,0-22.61-.95-22.71-.97h0Z\" fill=\"#2f2e43\"/><path d=\"m492.19,205.64l-66.51-2.81s-8.42,31.92,2.22,65.18l12.86,200.29h31.04l29.26-206.61-8.87-56.05h0Z\" fill=\"#2f2e43\"/><path d=\"m485.25,336.46c-4.65,0-9.72-1.14-14.73-2.26-3.71-.83-6.98-1.04-9.6-1.2-3.98-.25-7.13-.45-8.88-2.78-1.73-2.3-1.73-6.21,0-13.92,2.3-10.24,7.42-26.6,13.68-40.06,8.09-17.36,15.86-25.35,23.11-23.72,9.71,2.18,13.58,18.39,15.03,27.85,2.02,13.21,1.84,28.91-.44,39.07h0c-3.02,13.45-9.95,17.01-18.18,17.01h.01Zm1.77-81.13c-5.33,0-11.87,7.78-18.57,22.18-6.17,13.25-11.21,29.36-13.48,39.45-1.45,6.48-1.61,10.01-.53,11.46.92,1.22,3.33,1.38,6.66,1.58,2.73.17,6.13.38,10.07,1.27,15.66,3.51,25.45,4.79,29.32-12.48,4.15-18.5.99-60.35-12.32-63.34-.38-.09-.77-.13-1.16-.13h.01Z\" fill=\"#dfdfe0\"/><polygon points=\"548.58 460.81 399.43 461.79 376.26 451.13 389.76 313.42 403.34 314.11 543.07 321.22 548.58 460.81\" fill=\"#6c63ff\"/><polygon points=\"399.43 461.79 376.26 451.13 389.76 313.42 403.34 314.11 399.43 461.79\" fill=\"#272223\" isolation=\"isolate\" opacity=\".2\"/><path d=\"m487.5,311.06c-2.77,0-5.8-.68-8.78-1.35-2.21-.5-4.16-.62-5.73-.72-2.37-.15-4.25-.27-5.29-1.66-1.03-1.37-1.03-3.7,0-8.3,1.37-6.11,4.42-15.86,8.16-23.88,4.82-10.35,9.46-15.11,13.78-14.14,5.79,1.3,8.1,10.96,8.96,16.61,1.2,7.88,1.1,17.24-.26,23.29h0c-1.8,8.02-5.93,10.14-10.84,10.14h0Zm1.06-48.37c-3.18,0-7.08,4.64-11.07,13.23-3.68,7.9-6.69,17.5-8.03,23.52-.87,3.86-.96,5.97-.31,6.83.55.73,1.98.82,3.97.94,1.63.1,3.65.23,6,.76,9.33,2.09,15.17,2.85,17.48-7.44,2.47-11.03.59-35.98-7.34-37.76-.23-.05-.46-.08-.69-.08h-.01Z\" fill=\"#dfdfe0\"/><polygon points=\"525.25 385.21 436.33 385.79 422.51 379.44 430.56 297.33 438.66 297.74 521.97 301.98 525.25 385.21\" fill=\"#e2e3e4\"/><polygon points=\"436.33 385.79 422.51 379.44 430.56 297.33 438.66 297.74 436.33 385.79\" fill=\"#272223\" isolation=\"isolate\" opacity=\".2\"/><path id=\"uuid-2ebd868f-c256-4818-ab73-e4d3dd12d9e3-46-44-87-46-99-31\" d=\"m492.7,255.64c1.49,7.32-1.24,14.01-6.08,14.94s-9.97-4.26-11.45-11.58c-.63-2.92-.53-5.94.29-8.82l-5.89-31.11,15.22-2.41,4.19,30.92c1.89,2.36,3.16,5.12,3.72,8.06h0Z\" fill=\"#f8a8ab\"/><path d=\"m433,71.45s22.26-2.82,24.92,3.83,33.92,164.94,33.92,164.94h-20.62l-38.22-168.77s0,0,0,0Z\" fill=\"#e2e3e4\"/><polygon points=\"278.34 105.33 255.98 112.68 255.98 80.5 276.27 80.5 278.34 105.33\" fill=\"#f8a8ab\"/><circle cx=\"258.75\" cy=\"67.13\" r=\"22.28\" fill=\"#f8a8ab\"/><path d=\"m264.87,64.92c-3.73-.11-6.18-3.88-7.63-7.32s-2.94-7.39-6.4-8.81c-2.83-1.16-7.82,6.69-10.05,4.6-2.33-2.18-.06-13.37,2.41-15.38s5.85-2.4,9.03-2.55c7.76-.36,15.57.27,23.18,1.86,4.71.98,9.55,2.46,12.95,5.86,4.3,4.32,5.4,10.83,5.71,16.92.32,6.23-.04,12.75-3.07,18.2-3.03,5.45-9.37,9.47-15.45,8.08-.61-3.3.01-6.69.25-10.05.23-3.35-.01-6.97-2.06-9.64s-6.42-3.73-8.8-1.36\" fill=\"#2f2e43\"/><path d=\"m292.28,72.64c2.23-1.63,4.9-3,7.64-2.66,2.96.36,5.47,2.8,6.23,5.69s-.09,6.07-1.93,8.43c-1.83,2.36-4.56,3.92-7.44,4.7-1.67.45-3.5.64-5.09-.04-2.34-1.01-3.61-4-2.69-6.38\" fill=\"#2f2e43\"/><rect x=\"250.02\" y=\"463.43\" width=\"20.94\" height=\"29.71\" fill=\"#f8a8ab\"/><path d=\"m229.62,511.12c-2.2,0-4.16-.05-5.64-.19-5.56-.51-10.87-4.62-13.54-7.02-1.2-1.08-1.58-2.8-.96-4.28h0c.45-1.06,1.34-1.86,2.45-2.17l14.7-4.2,23.8-16.06.27.48c.1.18,2.44,4.39,3.22,7.23.3,1.08.22,1.98-.23,2.68-.31.48-.75.76-1.1.92.43.45,1.78,1.37,5.94,2.03,6.07.96,7.35-5.33,7.4-5.59l.04-.21.18-.12c2.89-1.86,4.67-2.71,5.28-2.53.38.11,1.02.31,2.75,17.44.17.54,1.38,4.48.56,8.25-.89,4.1-18.81,2.69-22.4,2.37-.1.01-13.52.97-22.71.97h-.01Z\" fill=\"#2f2e43\"/><rect x=\"319.09\" y=\"443.36\" width=\"20.94\" height=\"29.71\" transform=\"translate(-192.55 243.81) rotate(-31.95)\" fill=\"#f8a8ab\"/><path d=\"m306.98,507.05c-2.46,0-4.72-.3-6.33-.58-1.58-.28-2.82-1.54-3.08-3.12h0c-.18-1.14.15-2.29.93-3.14l10.25-11.34,11.7-26.22.48.26c.18.1,4.39,2.43,6.56,4.43.83.76,1.24,1.57,1.22,2.4-.01.58-.23,1.04-.45,1.37.6.16,2.23.22,6.11-1.42,5.66-2.39,3.42-8.41,3.32-8.66l-.08-.2.09-.19c1.47-3.11,2.52-4.77,3.14-4.94.39-.11,1.03-.28,11.56,13.35.43.36,3.54,3.07,4.84,6.7,1.41,3.95-14.54,12.24-17.75,13.86-.1.08-16.79,12.21-23.65,15.66-2.72,1.37-5.94,1.79-8.87,1.79h0Z\" fill=\"#2f2e43\"/><path d=\"m286.38,214.98h-58.63l-5.32,54.54,23.28,201.52h29.93l-11.97-116.39,48.55,105.08,26.6-18.62-37.91-98.1s13.54-85.46,2.9-106.75-17.43-21.28-17.43-21.28h0Z\" fill=\"#2f2e43\"/><polygon points=\"315.54 218.3 222.43 218.3 250.36 97.92 290.93 97.92 315.54 218.3\" fill=\"#6c63ff\"/><path id=\"uuid-f899ad7f-3d0f-4b30-ad3c-9c1473a48add-47-45-88-47-100-32\" d=\"m199.3,95.55c-1.49-7.32,1.24-14.01,6.08-14.94s9.97,4.26,11.45,11.58c.63,2.92.53,5.94-.29,8.82l5.89,31.11-15.22,2.41-4.19-30.92c-1.89-2.36-3.16-5.12-3.72-8.06h0Z\" fill=\"#f8a8ab\"/><path d=\"m289.94,97.92h-35.78l-24.12,48.24-9.1-36.15-19.99,2.12s4.73,70.63,25.4,68.24c20.67-2.39,68.88-66.02,63.58-82.46h.01Z\" fill=\"#6c63ff\"/><path d=\"m323.73,326.73c-2.77,0-5.8-.68-8.78-1.35-2.21-.5-4.16-.62-5.73-.72-2.37-.15-4.25-.27-5.29-1.66-1.03-1.37-1.03-3.7,0-8.3,1.37-6.11,4.42-15.86,8.16-23.88,4.82-10.35,9.46-15.11,13.78-14.14,5.79,1.3,8.1,10.96,8.96,16.61,1.2,7.88,1.1,17.24-.26,23.29h0c-1.8,8.02-5.93,10.14-10.84,10.14h0Zm1.06-48.37c-3.18,0-7.08,4.64-11.07,13.23-3.68,7.9-6.69,17.5-8.03,23.52-.87,3.86-.96,5.97-.31,6.83.55.73,1.98.82,3.97.94,1.63.1,3.65.23,6,.76,9.33,2.09,15.17,2.85,17.48-7.44,2.47-11.03.59-35.98-7.34-37.76-.23-.05-.46-.08-.69-.08h-.01Z\" fill=\"#dfdfe0\"/><polygon points=\"361.49 400.87 272.57 401.45 258.75 395.1 266.8 312.99 274.9 313.4 358.21 317.64 361.49 400.87\" fill=\"#e2e3e4\"/><polygon points=\"272.57 401.45 258.75 395.1 266.8 312.99 274.9 313.4 272.57 401.45\" fill=\"#272223\" isolation=\"isolate\" opacity=\".2\"/><path id=\"uuid-aa721d86-32e3-4ace-957f-0814f6d1eb89-48-46-89-48-101-33\" d=\"m329.89,281.37c1.49,7.32-1.24,14.01-6.08,14.94s-9.97-4.26-11.45-11.58c-.63-2.92-.53-5.94.29-8.82l-5.89-31.11,15.22-2.41,4.19,30.92c1.89,2.36,3.16,5.12,3.72,8.06h0Z\" fill=\"#f8a8ab\"/><path d=\"m269.54,97.92s20.33-.86,21.39,0c5.55,4.53,38.1,168.04,38.1,168.04h-20.62l-38.87-168.04s0,0,0,0Z\" fill=\"#6c63ff\"/></svg>")
	svgFileContent2 := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" width=\"537.64\" height=\"508.91\" viewBox=\"0 0 537.64 508.91\"><path d=\"m0,480.14c0,.66.53,1.19,1.19,1.19h535.26c.66,0,1.19-.53,1.19-1.19s-.53-1.19-1.19-1.19H1.19c-.66,0-1.19.53-1.19,1.19Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><path d=\"m160.71,21.3l-4.21,10.53s-3.88,14.74.67,20.62c4.54,5.87,5.55,50.88.67,54.2-4.88,3.33,61.85-30.82,61.85-30.82,0,0-26.05-60.41-26.49-60.63s-17.51-5.54-17.51-5.54l-14.96,11.64s-.02,0-.02,0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><polygon points=\"204.16 73.35 181.8 80.7 181.8 48.52 202.08 48.52 204.16 73.35\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path id=\"uuid-bb1e70bb-146f-43f8-9ef8-107ef6e1721c-44-89-87-44-44-105-99-34\" d=\"m135.81,256.05c-1.21,7.37-6.13,12.66-10.98,11.81-4.85-.85-7.81-7.52-6.59-14.9.44-2.95,1.61-5.75,3.41-8.14l5.54-31.17,15.08,3.16-7.07,30.39c.93,2.88,1.14,5.91.61,8.85h0Z\" fill=\"#f3a3a6\" stroke-width=\"0\"/><rect x=\"174.81\" y=\"433.64\" width=\"20.94\" height=\"29.71\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m154.42,481.33c-2.2,0-4.16-.05-5.64-.19-5.56-.51-10.87-4.62-13.54-7.02-1.2-1.08-1.58-2.8-.96-4.28h0c.45-1.06,1.34-1.86,2.45-2.17l14.7-4.2,23.8-16.06.27.48c.1.18,2.44,4.39,3.22,7.23.3,1.08.22,1.98-.23,2.68-.31.48-.75.76-1.1.92.43.45,1.78,1.37,5.94,2.03,6.07.96,7.35-5.33,7.4-5.59l.04-.21.18-.12c2.89-1.86,4.67-2.71,5.28-2.53.38.11,1.02.31,2.75,17.44.17.54,1.38,4.48.56,8.25-.89,4.1-18.81,2.69-22.4,2.37-.1.01-13.52.97-22.71.97h0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><rect x=\"243.89\" y=\"413.6\" width=\"20.94\" height=\"29.71\" transform=\"translate(-188.2 199.51) rotate(-31.95)\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m231.78,477.26c-2.46,0-4.72-.3-6.33-.58-1.58-.28-2.82-1.54-3.08-3.12h0c-.18-1.14.15-2.29.93-3.14l10.25-11.34,11.7-26.22.48.26c.18.1,4.39,2.43,6.56,4.43.83.76,1.24,1.57,1.22,2.4,0,.58-.23,1.04-.45,1.37.6.16,2.23.22,6.11-1.42,5.66-2.39,3.42-8.41,3.32-8.66l-.08-.2.09-.19c1.47-3.11,2.52-4.77,3.14-4.94.39-.11,1.03-.28,11.56,13.35.43.36,3.54,3.07,4.84,6.7,1.41,3.95-14.54,12.24-17.75,13.86-.1.08-16.79,12.21-23.65,15.66-2.72,1.37-5.94,1.79-8.87,1.79h0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><path d=\"m211.17,185.19h-58.63l-5.32,54.54,23.28,201.52h29.93l-11.97-116.39,48.55,105.08,26.6-18.62-37.91-98.1s13.54-85.46,2.9-106.75c-10.64-21.28-17.43-21.28-17.43-21.28h0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><polygon points=\"240.33 188.52 138.58 188.52 175.16 68.14 215.73 68.14 240.33 188.52\" fill=\"#6c63ff\" stroke-width=\"0\"/><path d=\"m181.56,68.15s-25.27-.67-27.93,5.99c-2.66,6.65-33.92,164.94-33.92,164.94h20.62l41.24-170.93h-.01Z\" fill=\"#6c63ff\" stroke-width=\"0\"/><circle cx=\"184.57\" cy=\"35.14\" r=\"22.28\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m188.89.53s26.82-6.87,35.03,28.82c0,0,.22,9.31,4.88,13.08s3.1,23.28,3.1,23.28c0,0,8.87,14.63,1.33,21.28,0,0-3.33,13.52,1.55,21.28s23.28,50.44-7.32,53.04c0,0-15.52-9.37-8.2-37.3s-.89-43.36-.89-43.36c0,0-23.5-8.52-22.17-26.7,1.33-18.18-15.08-36.14-16.18-35.91s-12.64-6.43-17.74,13.97l-5.46-.93S154.53-3.24,188.89.53Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><path id=\"uuid-eeb5b608-6ee1-4f81-854a-587534d39606-45-90-88-45-45-106-100-35\" d=\"m257.7,252.48c1.49,7.32-1.24,14.01-6.08,14.94-4.84.93-9.97-4.26-11.45-11.58-.63-2.92-.53-5.94.29-8.82l-5.89-31.11,15.22-2.41,4.19,30.92c1.89,2.36,3.16,5.12,3.72,8.06h0Z\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m198,68.29s22.26-2.82,24.92,3.83,33.92,164.94,33.92,164.94h-20.62l-38.22-168.77h0Z\" fill=\"#6c63ff\" stroke-width=\"0\"/><path id=\"uuid-88df1b5d-ca7e-43f8-926d-2353a0ed292e-46-91-89-46-46-107-101-36\" d=\"m258.58,233.97c-5.4,5.16-6.99,12.21-3.54,15.73,3.44,3.53,10.61,2.2,16.02-2.97,2.19-2.03,3.83-4.57,4.8-7.41l22.6-22.18-11.12-10.66-21.09,22.99c-2.9.86-5.52,2.4-7.65,4.49h-.02Z\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m267.39,223.59l41.71-53.46,21.95-57.45c2.4-6.27,7.45-11,13.87-12.97,6.42-1.97,13.26-.9,18.76,2.95,8.99,6.28,11.86,18.31,6.68,27.98l-33.83,63.13-.05.1-57.89,43.82-11.2-14.08v-.02Z\" fill=\"#6c63ff\" stroke-width=\"0\"/><polygon points=\"365.14 106.48 339.12 115.03 339.12 77.59 362.72 77.59 365.14 106.48\" fill=\"#f3a3a6\" stroke-width=\"0\"/><circle cx=\"342.34\" cy=\"62.03\" r=\"25.92\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m348.94,64.06l-3.57,4.34-6.46-11.31s-8.25.43-8.25-5.7-7.57-6.29-7.57-6.29c0,0-10.72-18.71,11.48-13.78,0,0,15.39-10.55,23.05-1.53,0,0,23.47,11.82,16.75,32.4l-11.16,21.21-2.53-4.8s1.53-20.16-11.74-14.54Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><rect x=\"351.34\" y=\"461.22\" width=\"20.94\" height=\"29.71\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m330.94,508.91c-2.2,0-4.16-.05-5.64-.19-5.56-.51-10.87-4.62-13.54-7.02-1.2-1.08-1.58-2.8-.96-4.28h0c.45-1.06,1.34-1.86,2.45-2.17l14.7-4.2,23.8-16.06.27.48c.1.18,2.44,4.39,3.22,7.23.3,1.08.22,1.98-.23,2.68-.31.48-.75.76-1.1.92.43.45,1.78,1.37,5.94,2.03,6.07.96,7.35-5.33,7.4-5.59l.04-.21.18-.12c2.89-1.86,4.67-2.71,5.28-2.53.38.11,1.02.31,2.75,17.44.17.54,1.38,4.48.56,8.25-.89,4.1-18.81,2.69-22.4,2.37-.1.01-13.52.97-22.71.97h0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><rect x=\"407.56\" y=\"419.96\" width=\"20.94\" height=\"29.71\" transform=\"translate(-181.23 366.24) rotate(-39.6)\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m415.19,479.74c-1.7,1.4-3.24,2.61-4.46,3.45-4.61,3.15-11.32,3.37-14.9,3.22-1.61-.07-3-1.15-3.47-2.69h0c-.33-1.11-.15-2.29.5-3.24l8.65-12.61,8.1-27.54.51.2c.19.07,4.67,1.83,7.09,3.52.92.64,1.43,1.39,1.53,2.21.07.57-.09,1.07-.26,1.42.62.07,2.24-.08,5.87-2.22,5.29-3.12,2.27-8.79,2.13-9.03l-.1-.19.06-.2c1.04-3.28,1.87-5.06,2.46-5.31.37-.16.98-.42,13.23,11.69.48.3,3.92,2.57,5.69,6,1.93,3.73-12.78,14.07-15.75,16.1-.07.07-9.8,9.36-16.88,15.22h0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><path d=\"m316.71,232.11h71.83l-15.96,237.43h-23.94l-31.92-237.43h0Z\" fill=\"#2f2e43\" stroke-width=\"0\"/><polygon points=\"331.34 245.19 316.71 232.11 324.02 362.69 405.83 440.5 426.23 423.65 378.56 361.8 331.34 245.19\" fill=\"#2f2e43\" stroke-width=\"0\"/><path d=\"m331.34,92.44l35.47-5.76,11.02,19.26c14,24.48,19.71,52.84,16.28,80.84l-5.57,45.33h-71.83l14.63-139.67s0,0,0,0Z\" fill=\"#6c63ff\" stroke-width=\"0\"/><path id=\"uuid-46bdddeb-8c3f-4fea-a5c7-33aa4ef4575e-47-92-90-47-47-108-102-37\" d=\"m258.87,243.34c-5.96,4.51-8.35,11.32-5.34,15.22s10.29,3.41,16.25-1.1c2.41-1.77,4.34-4.1,5.62-6.81l25-19.42-9.82-11.88-23.61,20.41c-2.98.52-5.76,1.75-8.12,3.58h.02Z\" fill=\"#f3a3a6\" stroke-width=\"0\"/><path d=\"m268.82,234.04l47.6-48.29,28.43-54.53c3.1-5.95,8.67-10.07,15.28-11.29,6.6-1.22,13.27.64,18.3,5.09,8.21,7.28,9.67,19.56,3.41,28.56l-40.89,58.8-.06.09-62.56,36.84-9.5-15.28h-.01Z\" fill=\"#6c63ff\" stroke-width=\"0\"/></svg>")
	file1, errFile := os.Create("./public/shopping.svg")
	if errFile != nil {
		t.Error("Couldn't create shopping.svg file")
	}
	file2, errFile := os.Create("./public/meeting.svg")
	if errFile != nil {
		t.Error("Couldn't create meeting.svg file")
	}
	svgFile1ContentSize, _ := file1.Write(svgFileContent1)
	svgFile2ContentSize, _ := file2.Write(svgFileContent2)

	defer func() {
		errFolder := os.RemoveAll("./public")
		if errFolder != nil {
			t.Error("Couldn't remove public folder")
		}
	}()

	// Run the Application and compare the fileType and fileSize
	app.httpRegistered = true
	app.httpServer.port = 8002

	go app.Run()
	time.Sleep(1 * time.Second)

	host := "http://localhost:8002"

	tests := []struct {
		desc                       string
		method                     string
		path                       string
		body                       []byte
		statusCode                 int
		expectedBody               string
		expectedBodyLength         int
		expectedResponseHeaderType string
	}{
		{
			desc:         "check static files",
			method:       http.MethodGet,
			path:         "/public/",
			statusCode:   http.StatusOK,
			expectedBody: "<a href=\"meeting.svg\">meeting.svg</a>\n",
		},
		{
			desc:                       "check file content hardhat.jpeg",
			method:                     http.MethodGet,
			path:                       "/public/meeting.svg",
			statusCode:                 http.StatusOK,
			expectedBodyLength:         svgFile2ContentSize,
			expectedResponseHeaderType: "image/svg+xml",
		},
		{
			desc:                       "check file content industrial.jpeg",
			method:                     http.MethodGet,
			path:                       "/public/shopping.svg",
			statusCode:                 http.StatusOK,
			expectedBodyLength:         svgFile1ContentSize,
			expectedResponseHeaderType: "image/svg+xml",
		},
		{
			desc:       "check public endpoint",
			method:     http.MethodGet,
			path:       "/public",
			statusCode: http.StatusNotFound,
		},
	}

	for it, tc := range tests {
		request, _ := http.NewRequest(tc.method, host+tc.path, bytes.NewBuffer(tc.body))
		request.Header.Set("Content-Type", "application/json")
		client := http.Client{}
		resp, err := client.Do(request)
		bodyBytes, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		body := string(bodyBytes)
		assert.Nil(t, err, "TEST[%d], Failed.\n%s", it, tc.desc)
		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed with Status Body.\n%s", it, tc.desc)
		if tc.expectedBody != "" {
			assert.Contains(t, body, tc.expectedBody, "TEST [%d], Failed with Expected Body. \n%s", it, tc.desc)
		}
		if tc.expectedBodyLength != 0 {
			contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
			assert.Equal(t, tc.expectedBodyLength, contentLength, "TEST [%d], Failed at Content-Length.\n %s", it, tc.desc)
		}
		if tc.expectedResponseHeaderType != "" {
			assert.Equal(t, tc.expectedResponseHeaderType, resp.Header.Get("Content-Type"), "TEST [%d], Failed at Expected Content-Type.\n%s", it, tc.desc)
		}
	}

}
