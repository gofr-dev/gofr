package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRun_ServerStartsListening(t *testing.T) {
	port := testutil.GetFreePort(t)

	// Create a mock router and add a new route
	router := &gofrHTTP.Router{}
	router.Add(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// adding registered routes for applying middlewares
	var registeredMethods []string

	_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		met, _ := route.GetMethods()
		for _, method := range met {
			if !contains(registeredMethods, method) { // Check for uniqueness before adding
				registeredMethods = append(registeredMethods, method)
			}
		}

		return nil
	})

	router.RegisteredRoutes = &registeredMethods

	// Create a mock container
	c := container.NewContainer(getConfigs(t))

	// Create an instance of httpServer
	server := &httpServer{
		router: router,
		port:   port,
	}

	// Start the server
	go server.run(c)

	// Wait for the server to start listening
	time.Sleep(100 * time.Millisecond)

	var netClient = &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	// Send a GET request to the server
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		fmt.Sprintf("http://localhost:%d", port), http.NoBody)
	resp, err := netClient.Do(req)

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST Failed.\n")

	resp.Body.Close()
}

func getConfigs(t *testing.T) config.Config {
	t.Helper()

	var configLocation string

	if _, err := os.Stat("./configs"); err == nil {
		configLocation = "./configs"
	}

	return config.NewEnvFile(configLocation, logging.NewLogger(logging.INFO))
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
	go server.run(c)

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
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
	go server.run(c)

	// Create a context with a timeout to test the shutdown
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
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

func TestValidateCertificateAndKeyFiles_Success(t *testing.T) {
	certFile := createTempCertFile(t)
	defer os.Remove(certFile)

	keyFile := createTempKeyFile(t)
	defer os.Remove(keyFile)

	err := validateCertificateAndKeyFiles(certFile, keyFile)

	require.NoError(t, err, "TestValidateCertificateAndKeyFiles_Success Failed!")
}

func TestValidateCertificateAndKeyFiles_Error(t *testing.T) {
	tests := []struct {
		name          string
		certFilePath  string
		keyFilePath   string
		expectedError error
	}{
		{
			name:          "Certificate file does not exist",
			certFilePath:  "non-existent-cert.pem",
			keyFilePath:   createTempKeyFile(t),
			expectedError: fmt.Errorf("%w : %v", errInvalidCertificateFile, "non-existent-cert.pem"),
		},
		{
			name:          "Key file does not exist",
			certFilePath:  createTempCertFile(t),
			keyFilePath:   "non-existent-key.pem",
			expectedError: fmt.Errorf("%w : %v", errInvalidKeyFile, "non-existent-key.pem"),
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCertificateAndKeyFiles(tc.certFilePath, tc.keyFilePath)

			require.Equal(t, tc.expectedError.Error(), err.Error(),
				"TestValidateCertificateAndKeyFiles_Error [%d] : %v Failed!", i, tc.name)
		})
	}
}

// Helper function to create a temporary key file.
func createTempKeyFile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "key-*.pem")
	if err != nil {
		t.Fatalf("could not create temp key file: %v", err)
	}

	defer f.Close()

	return f.Name()
}

// Helper function to create a temporary certificate file.
func createTempCertFile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "cert-*.pem")
	if err != nil {
		t.Fatalf("could not create temp cert file: %v", err)
	}

	defer f.Close()

	return f.Name()
}

func TestBodySizeLimitMiddleware_Integration(t *testing.T) {
	port := testutil.GetFreePort(t)

	tests := []struct {
		name           string
		bodySize       int
		maxBodySize    string
		expectedStatus int
		description    string
	}{
		{
			name:           "Request within limit",
			bodySize:       1024, // 1 KB
			maxBodySize:    "2048", // 2 KB
			expectedStatus: http.StatusOK,
			description:    "Should allow requests within the body size limit",
		},
		{
			name:           "Request exceeds limit",
			bodySize:       2048, // 2 KB
			maxBodySize:    "1024", // 1 KB
			expectedStatus: http.StatusRequestEntityTooLarge,
			description:    "Should reject requests exceeding the body size limit",
		},
		{
			name:           "Request at exact limit",
			bodySize:       1024, // 1 KB
			maxBodySize:    "1024", // 1 KB
			expectedStatus: http.StatusOK,
			description:    "Should allow requests at the exact body size limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := map[string]string{
				"HTTP_PORT":         fmt.Sprint(port),
				"LOG_LEVEL":         "INFO",
				"HTTP_MAX_BODY_SIZE": tt.maxBodySize,
			}

			c := container.NewContainer(config.NewMockConfig(cfg))
			app := New()
			app.container = c
			app.Config = config.NewMockConfig(cfg)

			app.POST("/test", func(ctx *Context) (any, error) {
				var body map[string]any
				if err := ctx.Bind(&body); err != nil {
					return nil, err
				}
				return "success", nil
			})

			go app.Run()
			time.Sleep(100 * time.Millisecond)

			body := make([]byte, tt.bodySize)
			for i := range body {
				body[i] = 'a'
			}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
				fmt.Sprintf("http://localhost:%d/test", port), bytes.NewReader(body))
			require.NoError(t, err, "Failed to create request")
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err, "Failed to make request")
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode, tt.description)

			if tt.expectedStatus == http.StatusRequestEntityTooLarge {
				var errorResponse map[string]any
				err := json.NewDecoder(resp.Body).Decode(&errorResponse)
				require.NoError(t, err)
				assert.Equal(t, "ERROR", errorResponse["status"])
				assert.Contains(t, errorResponse["message"], "request body too large")
			}
		})
	}
}
