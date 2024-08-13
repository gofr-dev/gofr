package gofr

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/version"
)

func Test_newContextSuccess(t *testing.T) {
	httpRequest, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "/test", bytes.NewBufferString(`{"key":"value"}`))
	httpRequest.Header.Set("content-type", "application/json")

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
	t.Setenv("HTTP_PORT", "8005")

	app := New()

	server := httptest.NewServer(app.httpServer.router)
	defer server.Close()

	app.WebSocket("/ws", func(ctx *Context) (interface{}, error) {
		err := ctx.WriteMessageToSocket("Hello! GoFr")
		if err != nil {
			return nil, err
		}

		// TODO: returning error here to close the connection to the websocket
		// as the websocket close error is not caught because we are using no bind function here.
		// this must not be necessary. We should put an actual check in handleWebSocketConnection method instead.
		return nil, &websocket.CloseError{Code: websocket.CloseNormalClosure, Text: "Error closing"}
	})

	go app.Run()

	wsURL := "ws" + server.URL[len("http"):] + "/ws"

	// Create a WebSocket client
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	defer resp.Body.Close()
	defer ws.Close()

	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	expectedResponse := "Hello! GoFr"
	assert.Equal(t, expectedResponse, string(message))
}
