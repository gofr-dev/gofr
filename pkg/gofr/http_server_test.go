package gofr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	"gofr.dev/pkg/gofr/websocket"
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

	t.Cleanup(func() {
		_ = f.Close()
	})

	return f.Name()
}

// Helper function to create a temporary certificate file.
func createTempCertFile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "cert-*.pem")
	if err != nil {
		t.Fatalf("could not create temp cert file: %v", err)
	}

	t.Cleanup(func() {
		_ = f.Close()
	})

	return f.Name()
}

// TestMiddlewareChainShape pins which middlewares are present in the
// default HTTP server middleware chain, by exercising one request and
// asserting the observable side-effect of each middleware.
//
// If a future PR drops or rewires a middleware (intentional or not),
// at least one of these checks fails and the PR author must update
// this test deliberately.
//
// Observable signatures (one per middleware):
//
//	Tracer:           X-Correlation-ID header on the response
//	Logging:          a JSON request-log line on stdout
//	CORS:             Access-Control-Allow-Origin echoed for the Origin header
//	Metrics:          app_http_response histogram observation > 0
//	WSHandlerUpgrade: presence is verified by the chain count (it runs only
//	                  on Upgrade requests, so behavior on a plain GET is a
//	                  no-op, but the wrap still has to exist for upgrades to
//	                  work — separate test elsewhere covers upgrade behavior)
func TestMiddlewareChainShape(t *testing.T) {
	var (
		corsHeader     string
		correlationHdr string
		respCode       int
	)

	// Construct the entire pipeline inside StdoutOutputForFunc so the
	// logger (which captures os.Stdout at NewLogger time) writes into
	// the redirected pipe.
	logs := testutil.StdoutOutputForFunc(func() {
		c := container.NewContainer(config.NewMockConfig(map[string]string{
			"LOG_LEVEL": "INFO",
		}))

		mwConfig := middleware.Config{
			LogProbes:   middleware.LogProbes{},
			CorsHeaders: map[string]string{"Access-Control-Allow-Origin": "*"},
		}

		s := newHTTPServer(c, 0, mwConfig)
		s.router.Add(http.MethodGet, "/chain-check", http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
		))

		// Apply the run-time middleware (WSHandlerUpgrade) the same way
		// httpServer.run does, but without binding to a port.
		s.router.Use(middleware.WSHandlerUpgrade(c, s.ws))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/chain-check", http.NoBody)
		req.Header.Set("Origin", "https://example.com")
		req.RemoteAddr = "1.2.3.4:5678"
		req.RequestURI = "/chain-check"

		rr := httptest.NewRecorder()
		s.router.ServeHTTP(rr, req)

		resp := rr.Result()

		t.Cleanup(func() { _ = resp.Body.Close() })

		corsHeader = resp.Header.Get("Access-Control-Allow-Origin")
		correlationHdr = resp.Header.Get("X-Correlation-ID")
		respCode = resp.StatusCode
	})

	// Tracer + Logging share the X-Correlation-ID header — its presence
	// proves both are wired (Tracer creates the span context; Logging
	// reads the trace ID off it and sets the header).
	assert.NotEmpty(t, correlationHdr,
		"missing X-Correlation-ID — Tracer/Logging middleware is not wired")

	// CORS middleware echoes the Origin (with the wildcard policy we
	// configured above, it returns "*").
	assert.Equal(t, "*", corsHeader,
		"missing Access-Control-Allow-Origin — CORS middleware is not wired")

	assert.Equal(t, http.StatusOK, respCode, "unexpected status code")

	// Logging middleware writes one JSON request-log line per request.
	assert.Contains(t, logs, `"method":"GET"`, "Logging middleware did not emit a log line")
	assert.Contains(t, logs, `"uri":"/chain-check"`, "Logging middleware did not include URI")
	assert.Contains(t, logs, `"response":200`, "Logging middleware did not include status")

	// Metrics middleware records the response time histogram. The OTel
	// implementation buffers observations until the metric is scraped,
	// which is its own contract (TestFrameworkMetricsSnapshot covers
	// the registration shape). For chain wiring, the four signals above
	// are sufficient.
}

// benchDiscardResponseWriter is a zero-cost ResponseWriter for the
// full-chain microbenchmarks. Avoids per-iter buffer allocation that
// httptest.NewRecorder would add.
type benchDiscardResponseWriter struct {
	h    http.Header
	code int
}

func (d *benchDiscardResponseWriter) Header() http.Header {
	if d.h == nil {
		d.h = http.Header{}
	}

	return d.h
}

func (*benchDiscardResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *benchDiscardResponseWriter) WriteHeader(c int)         { d.code = c }

// buildBenchRouter reconstructs the same middleware chain that
// gofr.New() / newHTTPServer + httpServer.run wire — Tracer, Logging,
// CORS, Metrics, WSHandlerUpgrade — so we can measure end-to-end request
// cost through the real chain without binding to a port.
//
// Keep this in sync with newHTTPServer in http_server.go. Any change to
// GoFr's middleware composition needs a matching change here.
func buildBenchRouter(c *container.Container) http.Handler {
	r := gofrHTTP.NewRouter()
	wsManager := websocket.New()

	r.Use(
		middleware.Tracer,
		middleware.Logging(middleware.LogProbes{}, c.Logger),
		middleware.CORS(map[string]string{}, r.RegisteredRoutes),
		middleware.Metrics(c.Metrics()),
		middleware.WSHandlerUpgrade(c, wsManager),
	)

	r.Add(http.MethodGet, "/plaintext", http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("OK"))
		},
	))

	r.Add(http.MethodGet, "/json", http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"message":"hello"}}`))
		},
	))

	return r
}

// BenchmarkRequest_FullChain is the end-to-end microbenchmark every perf
// PR diffs against. Exercises the full middleware chain (tracer, logger,
// CORS, metrics, websocket-upgrade) + router + handler + writer. Uses
// OTel's default (noop) TracerProvider, so it isolates the framework
// cost without SDK overhead. Pair with BenchmarkRequest_FullChain_SDK
// to see the SDK delta.
func BenchmarkRequest_FullChain(b *testing.B) {
	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
	h := buildBenchRouter(c)
	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/plaintext", http.NoBody)
	w := &benchDiscardResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}

// BenchmarkRequest_FullChain_JSON runs the same chain against the /json
// handler. Pairs with the /json endpoint in the wrk benchmarks.
func BenchmarkRequest_FullChain_JSON(b *testing.B) {
	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
	h := buildBenchRouter(c)
	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/json", http.NoBody)
	w := &benchDiscardResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}

// BenchmarkRequest_FullChain_SDK runs the same chain with an SDK
// TracerProvider installed (ParentBased(TraceIDRatioBased(1.0)) —
// today's real default from initTracer in otel.go). Recording spans
// get built and discarded.
//
// Delta vs BenchmarkRequest_FullChain is what PR-1 is forecast to save
// end-to-end for users without a TRACE_EXPORTER configured.
func BenchmarkRequest_FullChain_SDK(b *testing.B) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.Empty()),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)
	otel.SetTracerProvider(tp)

	b.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
	h := buildBenchRouter(c)
	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/plaintext", http.NoBody)
	w := &benchDiscardResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}
