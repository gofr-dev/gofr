package gofr

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	"gofr.dev/pkg/gofr/version"
	gofrWebsocket "gofr.dev/pkg/gofr/websocket"
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dummy", http.NoBody)
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

func TestContext_WriteMessageToSocket_NilConnection(t *testing.T) {
	testContainer, _ := container.NewMockContainer(t)

	testReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ws", http.NoBody)
	gofrReq := gofrHTTP.NewRequest(testReq)
	ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), gofrReq, testContainer)

	err := ctx.WriteMessageToSocket("test message")

	assert.ErrorIs(t, err, ErrConnectionNotFound)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

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

// BenchmarkContext_New measures the cost of constructing a fresh
// *gofr.Context per request via newContext. Today this allocates the
// Context struct itself plus a ContextLogger that re-extracts the
// traceID from the request context via trace.SpanFromContext.
//
// Future targets that move this number:
//   - PR-7 (cache traceID once on Context) — avoids re-extraction.
//   - Pool *gofr.Context (deferred — see FIXES.md Open Question).
func BenchmarkContext_New(b *testing.B) {
	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/bench", http.NoBody)
	r := gofrHTTP.NewRequest(req)
	w := gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = newContext(w, r, c)
	}
}

// TestNewHTTPContextMatchesNewContext pins that co-allocating the Context with
// its Request and Responder changes nothing observable: the same path params,
// query params and context are reachable either way.
func TestNewHTTPContextMatchesNewContext(t *testing.T) {
	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))

	newReq := func() *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/42?q=x", http.NoBody)

		return mux.SetURLVars(r, map[string]string{"id": "42"})
	}

	viaOld := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet),
		gofrHTTP.NewRequest(newReq()), c)
	viaNew := newHTTPContext(httptest.NewRecorder(), newReq(), c)

	require.Equal(t, viaOld.PathParam("id"), viaNew.PathParam("id"))
	require.Equal(t, "42", viaNew.PathParam("id"))
	require.Equal(t, viaOld.Param("q"), viaNew.Param("q"))
	require.Equal(t, "x", viaNew.Param("q"))
	require.NotNil(t, viaNew.Context)
	require.NotNil(t, viaNew.Container)
}

// TestNewHTTPContextResponderWired proves the private responder is reachable and
// writes through to the recorder it was built from.
func TestNewHTTPContextResponderWired(t *testing.T) {
	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
	rec := httptest.NewRecorder()

	ctx := newHTTPContext(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody), c)
	ctx.responder.Respond("hello", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "hello")
}

// dialRecordingWSServer starts a WebSocket server that forwards every message it receives to the
// returned channel, and returns a client connection to it.
func dialRecordingWSServer(t *testing.T) (conn *gofrWebsocket.Connection, received <-chan string) {
	t.Helper()

	messages := make(chan string, 1)
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		defer conn.Close()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			messages <- string(msg)
		}
	}))
	t.Cleanup(srv.Close)

	client, resp, err := websocket.DefaultDialer.Dial("ws"+srv.URL[len("http"):], nil)
	require.NoError(t, err)

	resp.Body.Close()
	t.Cleanup(func() { client.Close() })

	return &gofrWebsocket.Connection{Conn: client}, messages
}

func TestContext_WriteMessageToService_Cases(t *testing.T) {
	tests := []struct {
		desc    string
		service string
		data    any
		expErr  error
		expMsg  string
	}{
		{desc: "message delivered", service: "svc", data: map[string]string{"a": "b"}, expErr: nil, expMsg: `{"a":"b"}`},
		{desc: "unknown service", service: "other", data: "hi", expErr: ErrConnectionNotFound, expMsg: wsEndMarker},
		{desc: "unserializable data", service: "svc", data: make(chan int), expErr: ErrMarshalingResponse, expMsg: wsEndMarker},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			conn, received := dialRecordingWSServer(t)

			c := &container.Container{WSManager: gofrWebsocket.New()}
			c.AddConnection("svc", conn)

			req := gofrHTTP.NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
			ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), req, c)

			err := ctx.WriteMessageToService(tc.service, tc.data)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expMsg, firstMessageBeforeEnd(t, conn, received))
		})
	}
}

func TestContext_WriteMessageToSocket_Cases(t *testing.T) {
	tests := []struct {
		desc   string
		data   any
		expErr error
		expMsg string
	}{
		{desc: "message delivered", data: []byte("hello"), expErr: nil, expMsg: "hello"},
		{desc: "unserializable data", data: func() {}, expErr: ErrMarshalingResponse, expMsg: wsEndMarker},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			conn, received := dialRecordingWSServer(t)

			c := &container.Container{WSManager: gofrWebsocket.New()}

			reqCtx := context.WithValue(t.Context(), gofrWebsocket.WSConnectionKey, conn)
			req := gofrHTTP.NewRequest(httptest.NewRequestWithContext(reqCtx, http.MethodGet, "/", http.NoBody))
			ctx := newContext(gofrHTTP.NewResponder(httptest.NewRecorder(), http.MethodGet), req, c)

			err := ctx.WriteMessageToSocket(tc.data)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expMsg, firstMessageBeforeEnd(t, conn, received))
		})
	}
}

// wsEndMarker is written after the call under test; seeing it first means the call wrote nothing.
const wsEndMarker = "END"

// firstMessageBeforeEnd writes wsEndMarker on conn and returns the first message the server received.
// Messages on one connection arrive in order, so this is the call's message if it wrote one.
func firstMessageBeforeEnd(t *testing.T, conn *gofrWebsocket.Connection, received <-chan string) string {
	t.Helper()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(wsEndMarker)))

	select {
	case msg := <-received:
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("websocket server received nothing")

		return ""
	}
}

// recordedLog is a single call captured by recordingLogger.
type recordedLog struct {
	method string
	format string
	args   []any
}

// recordingLogger captures every call so tests can assert which logger received
// a message and which arguments (including the trace ID marker) were passed.
type recordingLogger struct {
	logs  []recordedLog
	level logging.Level
}

func (r *recordingLogger) recordf(method, format string, args ...any) {
	r.logs = append(r.logs, recordedLog{method: method, format: format, args: args})
}

func (r *recordingLogger) Debug(args ...any)             { r.recordf("Debug", "", args...) }
func (r *recordingLogger) Debugf(f string, args ...any)  { r.recordf("Debugf", f, args...) }
func (r *recordingLogger) Log(args ...any)               { r.recordf("Log", "", args...) }
func (r *recordingLogger) Logf(f string, args ...any)    { r.recordf("Logf", f, args...) }
func (r *recordingLogger) Info(args ...any)              { r.recordf("Info", "", args...) }
func (r *recordingLogger) Infof(f string, args ...any)   { r.recordf("Infof", f, args...) }
func (r *recordingLogger) Notice(args ...any)            { r.recordf("Notice", "", args...) }
func (r *recordingLogger) Noticef(f string, args ...any) { r.recordf("Noticef", f, args...) }
func (r *recordingLogger) Warn(args ...any)              { r.recordf("Warn", "", args...) }
func (r *recordingLogger) Warnf(f string, args ...any)   { r.recordf("Warnf", f, args...) }
func (r *recordingLogger) Error(args ...any)             { r.recordf("Error", "", args...) }
func (r *recordingLogger) Errorf(f string, args ...any)  { r.recordf("Errorf", f, args...) }
func (r *recordingLogger) Fatal(args ...any)             { r.recordf("Fatal", "", args...) }
func (r *recordingLogger) Fatalf(f string, args ...any)  { r.recordf("Fatalf", f, args...) }
func (r *recordingLogger) ChangeLevel(level logging.Level) {
	r.level = level
	r.recordf("ChangeLevel", "")
}

func tracedTestContext() (ctx context.Context, traceID string) {
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19},
		SpanID:     oteltrace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		TraceFlags: oteltrace.FlagsSampled,
	})

	return oteltrace.ContextWithSpanContext(context.Background(), sc), sc.TraceID().String()
}

// countTraceMarkers counts the args that render as the given trace ID; the marker
// type is unexported in the logging package, so it is matched by its value.
func countTraceMarkers(args []any, traceID string) int {
	n := 0

	for _, a := range args {
		if fmt.Sprint(a) == traceID {
			n++
		}
	}

	return n
}

// callAllLogMethods invokes every logging.Logger method once on ctx.
func callAllLogMethods(ctx *Context) {
	ctx.Debug("m")
	ctx.Debugf("m %s", "a")
	ctx.Log("m")
	ctx.Logf("m %s", "a")
	ctx.Info("m")
	ctx.Infof("m %s", "a")
	ctx.Notice("m")
	ctx.Noticef("m %s", "a")
	ctx.Warn("m")
	ctx.Warnf("m %s", "a")
	ctx.Error("m")
	ctx.Errorf("m %s", "a")
	ctx.Fatal("m")
	ctx.Fatalf("m %s", "a")
}

func allLogMethods() []string {
	return []string{
		"Debug", "Debugf", "Log", "Logf", "Info", "Infof", "Notice", "Noticef",
		"Warn", "Warnf", "Error", "Errorf", "Fatal", "Fatalf",
	}
}

func methodsOf(logs []recordedLog) []string {
	out := make([]string, 0, len(logs))
	for _, l := range logs {
		out = append(out, l.method)
	}

	return out
}

// Hand-built contexts (tests, background jobs, gofr-cli generated gRPC wrappers)
// leave ContextLogger zero-valued. Logging through the context must fall back to
// the container's logger instead of dereferencing a nil base logger.
func TestContext_Logging_HandBuiltContextFallsBackToContainerLogger(t *testing.T) {
	tracedCtx, traceID := tracedTestContext()

	tests := []struct {
		desc        string
		stdCtx      context.Context
		wantMarkers int
	}{
		{desc: "background context logs without a trace marker", stdCtx: context.Background(), wantMarkers: 0},
		{desc: "traced context keeps the trace ID", stdCtx: tracedCtx, wantMarkers: 1},
		{desc: "nil context logs without a trace marker", stdCtx: nil, wantMarkers: 0},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			rec := &recordingLogger{}
			ctx := &Context{Context: tc.stdCtx, Container: &container.Container{Logger: rec}}

			require.NotPanics(t, func() { callAllLogMethods(ctx) })

			require.Equal(t, allLogMethods(), methodsOf(rec.logs))

			for _, l := range rec.logs {
				assert.Equal(t, tc.wantMarkers, countTraceMarkers(l.args, traceID), "method %s", l.method)
			}
		})
	}
}

// The gRPC wrapper generated by gofr-cli builds the context with exactly these
// fields; ctx.Errorf in a gRPC handler must not panic.
func TestContext_Logging_GRPCWrapperShapedContext(t *testing.T) {
	tracedCtx, traceID := tracedTestContext()
	rec := &recordingLogger{}

	ctx := &Context{
		Context:   tracedCtx,
		Container: &container.Container{Logger: rec},
		Request:   nil,
	}

	require.NotPanics(t, func() { ctx.Errorf("failed: %v", "boom") })
	require.Len(t, rec.logs, 1)
	assert.Equal(t, "Errorf", rec.logs[0].method)
	assert.Equal(t, "failed: %v", rec.logs[0].format)
	assert.Equal(t, 1, countTraceMarkers(rec.logs[0].args, traceID))
}

// Framework-built contexts must keep logging through their ContextLogger: one
// trace marker per call, delivered to the logger the ContextLogger wraps.
func TestContext_Logging_FrameworkContextUsesContextLogger(t *testing.T) {
	tracedCtx, traceID := tracedTestContext()

	containerLog := &recordingLogger{}
	ctxLog := &recordingLogger{}

	ctx := &Context{
		Context:       tracedCtx,
		Container:     &container.Container{Logger: containerLog},
		ContextLogger: logging.ContextLoggerFor(tracedCtx, ctxLog),
	}

	callAllLogMethods(ctx)

	assert.Empty(t, containerLog.logs, "an initialized ContextLogger must not be bypassed")
	require.Equal(t, allLogMethods(), methodsOf(ctxLog.logs))

	for _, l := range ctxLog.logs {
		assert.Equal(t, 1, countTraceMarkers(l.args, traceID), "method %s", l.method)
	}
}

func TestContext_Logging_NewHTTPContextKeepsSingleTraceMarker(t *testing.T) {
	tracedCtx, traceID := tracedTestContext()
	rec := &recordingLogger{}

	req := httptest.NewRequestWithContext(tracedCtx, http.MethodGet, "/", http.NoBody)
	ctx := newHTTPContext(httptest.NewRecorder(), req, &container.Container{Logger: rec})

	ctx.Infof("hello %s", "world")

	require.Len(t, rec.logs, 1)
	assert.Equal(t, 1, countTraceMarkers(rec.logs[0].args, traceID))
}

func TestContext_ChangeLevel(t *testing.T) {
	t.Run("hand-built context changes the container logger level", func(t *testing.T) {
		rec := &recordingLogger{}
		ctx := &Context{Context: context.Background(), Container: &container.Container{Logger: rec}}

		require.NotPanics(t, func() { ctx.ChangeLevel(logging.WARN) })
		assert.Equal(t, logging.WARN, rec.level)
	})

	t.Run("framework context changes the wrapped logger level", func(t *testing.T) {
		rec := &recordingLogger{}
		ctx := newContext(nil, &noopRequest{}, &container.Container{Logger: rec})

		ctx.ChangeLevel(logging.ERROR)
		assert.Equal(t, logging.ERROR, rec.level)
	})
}

type discardLogger struct{}

func (discardLogger) Debug(...any)              {}
func (discardLogger) Debugf(string, ...any)     {}
func (discardLogger) Log(...any)                {}
func (discardLogger) Logf(string, ...any)       {}
func (discardLogger) Info(...any)               {}
func (discardLogger) Infof(string, ...any)      {}
func (discardLogger) Notice(...any)             {}
func (discardLogger) Noticef(string, ...any)    {}
func (discardLogger) Warn(...any)               {}
func (discardLogger) Warnf(string, ...any)      {}
func (discardLogger) Error(...any)              {}
func (discardLogger) Errorf(string, ...any)     {}
func (discardLogger) Fatal(...any)              {}
func (discardLogger) Fatalf(string, ...any)     {}
func (discardLogger) ChangeLevel(logging.Level) {}

// Every logging.Logger method must work on a hand-built Context. A method added to the interface
// later but not defined on *Context would be promoted from the zero ContextLogger and panic.
func TestContext_HandBuiltContextHandlesEveryLoggerMethod(t *testing.T) {
	ctx := reflect.ValueOf(&Context{Context: t.Context(), Container: &container.Container{Logger: discardLogger{}}})
	loggerType := reflect.TypeFor[logging.Logger]()

	for i := range loggerType.NumMethod() {
		name := loggerType.Method(i).Name
		method := ctx.MethodByName(name)
		args := make([]reflect.Value, method.Type().NumIn())

		for j := range args {
			args[j] = reflect.Zero(method.Type().In(j))
		}

		// Fatal and Fatalf are included: the discard logger does not exit the process.
		assert.NotPanics(t, func() { method.Call(args) }, name)
	}
}

// The initialization check must not become part of Context's API: an exported method on the
// embedded ContextLogger would be promoted onto every *Context.
func TestContext_DoesNotExposeIsInitialized(t *testing.T) {
	_, ok := reflect.TypeFor[*Context]().MethodByName("IsInitialized")
	assert.False(t, ok, "IsInitialized must not be promoted onto *gofr.Context")
}

func BenchmarkContext_Infof(b *testing.B) {
	tracedCtx, _ := tracedTestContext()
	req := httptest.NewRequestWithContext(tracedCtx, http.MethodGet, "/", http.NoBody)
	c := &container.Container{Logger: discardLogger{}}

	b.Run("framework context", func(b *testing.B) {
		ctx := newHTTPContext(httptest.NewRecorder(), req, c)

		b.ReportAllocs()

		for b.Loop() {
			ctx.Infof("hello %s", "world")
		}
	})

	b.Run("hand-built context", func(b *testing.B) {
		ctx := &Context{Context: tracedCtx, Container: c}

		b.ReportAllocs()

		for b.Loop() {
			ctx.Infof("hello %s", "world")
		}
	})
}
