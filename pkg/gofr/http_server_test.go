package gofr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

func TestRun_ServerStartsListening(t *testing.T) {
	// Create a mock router and add a new route
	router := &gofrHTTP.Router{}
	router.Add(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create a mock container
	c := &container.Container{
		Logger: logging.NewLogger(logging.INFO),
	}

	// Create an instance of httpServer
	server := &httpServer{
		router: router,
		port:   8080,
	}

	// Start the server
	go server.Run(c)

	// Wait for the server to start listening
	time.Sleep(1 * time.Second)

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// Send a GET request to the server
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080", http.NoBody)
	resp, err := netClient.Do(req)

	assert.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST Failed.\n")

	resp.Body.Close()
}

func TestRegisterProfillingRoutes(t *testing.T) {
	c := &container.Container{
		Logger: logging.NewLogger(logging.INFO),
	}

	server := &httpServer{
		router: gofrHTTP.NewRouter(),
		port:   8080,
	}

	server.RegisterProfilingRoutes()

	server.Run(c)

	// Test if the expected handlers are registered for the pprof endpoints
	expectedRoutes := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/symbol",
	}

	serverURL := "http://localhost:" + strconv.Itoa(8000)

	for _, route := range expectedRoutes {
		r := httptest.NewRequest(http.MethodGet, serverURL+route, http.NoBody)
		rr := httptest.NewRecorder()
		server.router.ServeHTTP(rr, r)

		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func TestServer_ShutDown(t *testing.T) {
	// Create a mock router and add a new route
	router := &gofrHTTP.Router{}
	router.Add(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create a mock container
	c := &container.Container{
		Logger: logging.NewLogger(logging.INFO),
	}

	// Create an instance of httpServer
	server := &httpServer{
		router: router,
		port:   8080,
	}

	// Start the server
	go server.Run(c)

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Ensure the server is running by making a request
	resp, err := http.Get("http://localhost:8080/")
	assert.NoError(t, err, "Server did not start as expected")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shut down the server
	err = server.Shutdown(context.Background())
	assert.NoError(t, err, "Server did not shut down as expected")

	// Ensure the server is no longer running
	resp, err = http.Get("http://localhost:8080/")
	assert.Error(t, err, "Expected error when server is shut down")
	assert.Nil(t, resp, "Response should be nil when server is shut down")
}
