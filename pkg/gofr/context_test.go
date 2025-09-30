package gofr

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	"gofr.dev/pkg/gofr/version"
)

func Test_newContextSuccess(t *testing.T) {
	httpRequest, err := http.NewRequestWithContext(t.Context(),
		http.MethodPost, "/test", bytes.NewBufferString(`{"key":"value"}`))
	httpRequest.Header.Set("Content-Type", "application/json")

	if err != nil {
		t.Fatalf("unable to create request with context %v", err)
	}

	req := gofrHTTP.NewRequest(httpRequest)

	ctx := newContext(nil, req, container.NewContainer(config.NewEnvFile("",
		logging.NewMockLogger(logging.DEBUG))))

	body := map[string]string{}

	err = ctx.Bind(&body)

	assert.Equal(t, map[string]string{"key": "value"}, body, "TEST Failed \n unable to read body")
	require.NoError(t, err, "TEST Failed \n unable to read body")
}

func TestContext_AddTrace(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	tr := otel.GetTracerProvider().Tracer("gofr-" + version.Framework)

	// Creating a dummy request with trace
	req := httptest.NewRequest(http.MethodGet, "/dummy", http.NoBody)
	originalCtx, span := tr.Start(req.Context(), "start")

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	// Creating a new context from original context and adding trace
	ctx := Context{
		Context: originalCtx,
	}

	newSpan := ctx.Trace("Some Work")
	defer newSpan.End()

	newtraceID := newSpan.SpanContext().TraceID().String()
	newSpanID := newSpan.SpanContext().SpanID().String()

	// both traceIDs must be same as context is same
	assert.Equal(t, traceID, newtraceID)
	// spanIDs must not be same
	assert.NotEqual(t, spanID, newSpanID)
}

func TestContext_WriteMessageToSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Use NewServerConfigs to get free ports for both HTTP and metrics
	configs := testutil.NewServerConfigs(t)

	app := New()
	messageChan := make(chan string, 1)
	handlerDone := make(chan struct{})

	var handlerOnce sync.Once

	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		defer handlerOnce.Do(func() { close(handlerDone) })

		return handleWebSocketMessage(ctx, messageChan)
	})

	// Start server in goroutine
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		app.Run()
	}()

	// Give server time to start
	time.Sleep(30 * time.Millisecond)

	wsURL := fmt.Sprintf("ws://localhost:%d/ws", configs.HTTPPort)

	// Test the WebSocket connection
	// Note: We don't wait for server to stop as it's designed to run until signal.
	// The test completes after testing the WebSocket functionality.
	testWebSocketConnection(t, wsURL, messageChan, handlerDone)
}

// handleWebSocketMessage handles the WebSocket message sending logic.
func handleWebSocketMessage(ctx *Context, messageChan chan string) (any, error) {
	err := ctx.WriteMessageToSocket("Hello! GoFr")
	if err != nil {
		// Signal error instead of calling t.Errorf in goroutine
		select {
		case messageChan <- "ERROR":
		default:
		}

		return nil, err
	}

	// Signal that message was sent
	select {
	case messageChan <- "Hello! GoFr":
	default:
	}

	return "Hello! GoFr", nil
}

// testWebSocketConnection tests the WebSocket connection and message reading.
func testWebSocketConnection(t *testing.T, wsURL string, messageChan chan string, handlerDone chan struct{}) {
	t.Helper()
	// Create WebSocket client with timeout
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	ws, resp, err := dialer.Dial(wsURL, nil)

	require.NoError(t, err, "WebSocket handshake failed")

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		if ws != nil {
			ws.Close()
		}
	}()

	// Set read deadline and read message
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := ws.ReadMessage()
	require.NoError(t, err, "Failed to read WebSocket message")

	assert.Equal(t, "Hello! GoFr", string(message))

	// Wait for handler completion
	select {
	case msg := <-messageChan:
		if msg == "ERROR" {
			t.Error("WriteMessageToSocket failed in handler")
		} else {
			assert.Equal(t, "Hello! GoFr", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out waiting for handler completion")
	}

	// Wait for handler to complete before closing connection
	select {
	case <-handlerDone:
		// Handler completed successfully
	case <-time.After(2 * time.Second):
		t.Error("Handler did not complete within timeout")
	}

	// Close the websocket connection to trigger cleanup
	ws.Close()

	// Wait a bit for cleanup to complete
	time.Sleep(10 * time.Millisecond)
}

func TestContext_WriteMessageToService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Use NewServerConfigs to get free ports for both HTTP and metrics
	configs := testutil.NewServerConfigs(t)

	app := New()

	// Create a simple echo server for testing
	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		// This is a simple echo server that reads a message and echoes it back
		var message string

		// Read the incoming message using ctx.Bind
		err := ctx.Bind(&message)
		if err != nil {
			return nil, err
		}

		// Echo the message back
		return message, nil
	})

	// Start server in goroutine
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		app.Run()
	}()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	wsURL := fmt.Sprintf("ws://localhost:%d/ws", configs.HTTPPort)

	// Establish a WebSocket connection to the echo server
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Dial should not return an error")

	defer func() {
		ws.Close()

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	// WebSocket service communication should be handled through the Context

	// Send a message to the echo server and read the response
	err = ws.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!"))
	require.NoError(t, err, "WriteMessage should not return an error")

	// Read the response
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := ws.ReadMessage()
	require.NoError(t, err, "ReadMessage should not return an error")

	assert.Equal(t, "Hello, WebSocket!", string(message))

	// Close the websocket connection to trigger cleanup
	ws.Close()

	// Wait a bit for cleanup to complete
	time.Sleep(10 * time.Millisecond)
}

func TestGetAuthInfo_BasicAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	ctx := context.WithValue(req.Context(), middleware.Username, "validUser")
	*req = *req.Clone(ctx)

	mockContainer, _ := container.NewMockContainer(t)
	gofrRq := gofrHTTP.NewRequest(req)

	c := &Context{
		Context:   ctx,
		Request:   gofrRq,
		Container: mockContainer,
	}

	res := c.GetAuthInfo().GetUsername()

	assert.Equal(t, "validUser", res)
}

func TestGetAuthInfo_ApiKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	ctx := context.WithValue(req.Context(), middleware.APIKey, "9221e451-451f-4cd6-a23d-2b2d3adea9cf")

	*req = *req.Clone(ctx)
	gofrRq := gofrHTTP.NewRequest(req)

	mockContainer, _ := container.NewMockContainer(t)

	c := &Context{
		Context:   ctx,
		Request:   gofrRq,
		Container: mockContainer,
	}

	res := c.GetAuthInfo().GetAPIKey()

	assert.Equal(t, "9221e451-451f-4cd6-a23d-2b2d3adea9cf", res)
}

func TestGetAuthInfo_JWTClaims(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":   "1234567890",
		"name":  "John Doe",
		"admin": true,
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	ctx := context.WithValue(req.Context(), middleware.JWTClaim, claims)

	*req = *req.Clone(ctx)
	gofrRq := gofrHTTP.NewRequest(req)

	mockContainer, _ := container.NewMockContainer(t)

	c := &Context{
		Context:   ctx,
		Request:   gofrRq,
		Container: mockContainer,
	}

	res := c.GetAuthInfo().GetClaims()

	assert.Equal(t, claims, res)
}

func TestContext_GetCorrelationID(t *testing.T) {
	// Setup OpenTelemetry tracer
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer("test")

	t.Run("with span", func(t *testing.T) {
		ctx, span := tracer.Start(t.Context(), "test-span")
		defer span.End()

		gofCtx := &Context{Context: ctx}
		correlationID := gofCtx.GetCorrelationID()

		assert.Len(t, correlationID, 32, "Expected correlation ID length 32, got %d", len(correlationID))
		assert.NotEqual(t, "00000000000000000000000000000000", correlationID, "Expected non-empty correlation ID")
	})

	t.Run("without span", func(t *testing.T) {
		gofCtx := &Context{Context: t.Context()}
		correlationID := gofCtx.GetCorrelationID()

		expected := "00000000000000000000000000000000"
		assert.Equal(t, expected, correlationID, "Expected empty TraceID when no span present")
	})
}
