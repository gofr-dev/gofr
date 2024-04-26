package gofr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

		app.Migrate(map[int64]migration.Migrate{1: {UP: func(d migration.Datasource) error {
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
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	c := container.NewContainer(config.NewMockConfig(nil))

	// Initialize a new App instance
	a := &App{
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(c),
		},
		container: c,
	}

	a.httpServer.router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		{"gofr exporter", mockConfig3, "Exporting traces to gofr at https://tracer.gofr.dev"},
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
