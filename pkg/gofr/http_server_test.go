package gofr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	require.NoError(t, err, "TEST Failed.\n")

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

func TestShutdown_ServerStopsListening(t *testing.T) {
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

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		errChan <- server.Shutdown(ctx)
	}()

	err := <-errChan

	require.NoError(t, err, "TEST Failed.\n")
}

func TestShutdown_ServerContextDeadline(t *testing.T) {
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

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Simulate a delay in the shutdown process to trigger context timeout
	shutdownCh := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond) // Delay longer than the context timeout
		shutdownCh <- server.Shutdown(ctx)
	}()

	err := <-shutdownCh

	require.ErrorIs(t, err, context.DeadlineExceeded, "Expected context deadline exceeded error")
}
