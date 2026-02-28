package gofr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

const (
	sseHeartbeatInterval = 15 * time.Second
	sseEventBufferSize   = 64
)

// SSEHandler is the function signature for SSE endpoints.
type SSEHandler func(ctx *Context, stream *SSEStream) error

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Name  string
	Data  any
	ID    string
	Retry int
}

// SSEStream decouples event production from network I/O via a buffered channel.
type SSEStream struct {
	events chan string
	done   chan struct{} // closed when ServeHTTP exits
}

func newSSEStream() *SSEStream {
	return &SSEStream{
		events: make(chan string, sseEventBufferSize),
		done:   make(chan struct{}),
	}
}

var (
	errStreamClosed          = errors.New("client disconnected: stream closed")
	errStreamingNotSupported = errors.New("streaming not supported: ResponseWriter does not implement http.Flusher")
	errHandlerPanicked       = errors.New("SSE handler panicked")
)

// Send enqueues a formatted SSE event.
func (s *SSEStream) Send(event SSEEvent) error {
	raw, err := formatEvent(event)
	if err != nil {
		return err
	}

	select {
	case s.events <- raw:
		return nil
	case <-s.done:
		return errStreamClosed
	}
}

// SendData is shorthand for Send(SSEEvent{Data: data}).
func (s *SSEStream) SendData(data any) error {
	return s.Send(SSEEvent{Data: data})
}

// SendEvent is shorthand for Send(SSEEvent{Name: name, Data: data}).
func (s *SSEStream) SendEvent(name string, data any) error {
	return s.Send(SSEEvent{Name: name, Data: data})
}

// SendComment enqueues an SSE comment (: prefix).
func (s *SSEStream) SendComment(text string) error {
	var sb strings.Builder

	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(&sb, ": %s\n", line)
	}

	sb.WriteString("\n")

	select {
	case s.events <- sb.String():
		return nil
	case <-s.done:
		return errStreamClosed
	}
}

// formatEvent builds the wire-format string for one SSE event.
func formatEvent(event SSEEvent) (string, error) {
	var sb strings.Builder

	if event.ID != "" {
		fmt.Fprintf(&sb, "id: %s\n", event.ID)
	}

	if event.Name != "" {
		fmt.Fprintf(&sb, "event: %s\n", event.Name)
	}

	if event.Retry > 0 {
		fmt.Fprintf(&sb, "retry: %d\n", event.Retry)
	}

	dataStr, err := formatSSEData(event.Data)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(dataStr, "\n") {
		fmt.Fprintf(&sb, "data: %s\n", line)
	}

	sb.WriteString("\n")

	return sb.String(), nil
}

// formatSSEData converts data to a string for SSE.
// Strings and []byte pass through; everything else is JSON-encoded.
func formatSSEData(data any) (string, error) {
	switch v := data.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal SSE data: %w", err)
		}

		return string(b), nil
	}
}

// sseHTTPHandler implements http.Handler for SSE endpoints.
type sseHTTPHandler struct {
	function  SSEHandler
	container *container.Container
}

var _ http.Handler = sseHTTPHandler{}

func (h sseHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if err := rc.Flush(); err != nil {
		ctx := newContext(gofrHTTP.NewResponder(w, r.Method), gofrHTTP.NewRequest(r), h.container)
		ctx.responder.Respond(nil, errStreamingNotSupported)

		return
	}

	traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()

	stream := newSSEStream()
	defer close(stream.done)

	ctx := newContext(gofrHTTP.NewResponder(w, r.Method), gofrHTTP.NewRequest(r), h.container)
	ctx.Context = r.Context()

	handlerDone := make(chan error, 1)

	go func() {
		defer func() {
			if re := recover(); re != nil {
				h.container.Logger.Errorf("SSE handler panicked: %v", re)

				handlerDone <- errHandlerPanicked
			}

			close(stream.events)
		}()

		handlerDone <- h.function(ctx, stream)
	}()

	h.drainLoop(w, r, rc, stream, handlerDone, traceID)
}

func (h sseHTTPHandler) drainLoop(
	w http.ResponseWriter,
	r *http.Request,
	rc *http.ResponseController,
	stream *SSEStream,
	handlerDone <-chan error,
	traceID string,
) {
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			h.container.Logger.Debugf("SSE client disconnected: traceID=%s", traceID)
			return

		case msg, ok := <-stream.events:
			if !ok {
				if err := <-handlerDone; err != nil {
					h.logError(traceID, err)
				}

				return
			}

			if _, err := fmt.Fprint(w, msg); err != nil {
				return
			}

			_ = rc.Flush()

		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}

			_ = rc.Flush()
		}
	}
}

func (h sseHTTPHandler) logError(traceID string, err error) {
	if err == nil {
		return
	}

	errorLog := &ErrorLogEntry{TraceID: traceID, Error: err.Error()}
	loggerHelper := h.container.Logger.Error

	switch logging.GetLogLevelForError(err) {
	case logging.ERROR:
	case logging.INFO:
		loggerHelper = h.container.Logger.Info
	case logging.NOTICE:
		loggerHelper = h.container.Logger.Notice
	case logging.DEBUG:
		loggerHelper = h.container.Logger.Debug
	case logging.WARN:
		loggerHelper = h.container.Logger.Warn
	case logging.FATAL:
		loggerHelper = h.container.Logger.Fatal
	}

	loggerHelper(errorLog)
}

// SSE registers a GET handler for Server-Sent Events on the given pattern.
func (a *App) SSE(pattern string, handler SSEHandler) {
	if !a.httpRegistered && !isPortAvailable(a.httpServer.port) {
		a.container.Logger.Fatalf("http port %d is blocked or unreachable", a.httpServer.port)
	}

	a.httpRegistered = true

	a.httpServer.router.Add("GET", pattern, sseHTTPHandler{
		function:  handler,
		container: a.container,
	})
}
