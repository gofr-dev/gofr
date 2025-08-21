package gofr

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	port := testutil.GetFreePort(t)
	t.Setenv("HTTP_PORT", fmt.Sprint(port))

	app := New()

	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		socketErr := ctx.WriteMessageToSocket("Hello! GoFr")
		if socketErr != nil {
			return nil, socketErr
		}

		conn := ctx.GetConnectionFromContext(ctx)
		defer conn.Close()

		return "", socketErr
	})

	go app.Run()
	time.Sleep(100 * time.Millisecond) // Wait for the server to boot

	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "WebSocket handshake failed")

	defer resp.Body.Close()
	defer ws.Close()

	_, message, err := ws.ReadMessage()
	require.NoError(t, err, "Failed to read WebSocket message")

	assert.Equal(t, "Hello! GoFr", string(message))
}

func TestContext_WriteMessageToService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	port := testutil.GetFreePort(t)
	t.Setenv("HTTP_PORT", fmt.Sprint(port))

	app := New()

	// Start a WebSocket server
	app.WebSocket("/ws", func(ctx *Context) (any, error) {
		conn := ctx.GetWSConnectionByServiceName("test-service")

		messageToSend := "Hello, WebSocket!"

		err := ctx.WriteMessageToService("test-service", messageToSend)
		if err != nil {
			return nil, err
		}

		_, receivedMessage, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}

		assert.Equal(t, messageToSend, string(receivedMessage))

		return nil, nil
	})

	go app.Run()
	time.Sleep(100 * time.Millisecond)

	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)

	serviceName := "test-service"
	retryInterval := 50 * time.Millisecond
	err := app.AddWSService(serviceName, wsURL, http.Header{}, true, retryInterval)
	require.NoError(t, err, "AddWSService should not return an error")

	// Establish a WebSocket connection
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Dial should not return an error")

	defer ws.Close()
	defer resp.Body.Close()

	require.NoError(t, err, "WebSocket handshake failed")
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
