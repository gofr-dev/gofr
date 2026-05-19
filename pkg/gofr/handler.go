package gofr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/static"
)

const colorCodeError = 202 // 202 is red color code

type Handler func(c *Context) (any, error)

/*
Developer Note: There is an implementation where we do not need this internal handler struct
and directly use Handler. However, in that case the container dependency is not injected and
has to be created inside ServeHTTP method, which will result in multiple unnecessary calls.
This is what we implemented first.

There is another possibility where we write our own Router implementation and let httpServer
use that router which will return a Handler and httpServer will then create the context with
injecting container and call that Handler with the new context. A similar implementation is
done in CMD. Since this will require us to write our own router - we are not taking that path
for now. In the future, this can be considered as well if we are writing our own HTTP router.
*/

type handler struct {
	function       Handler
	container      *container.Container
	requestTimeout time.Duration
}

// handlerOutcome carries the (result, error) pair from the per-request
// handler goroutine back to ServeHTTP. Sending it through a buffered
// channel keeps the goroutine's writes invisible to the main goroutine
// until the channel receive completes — no shared writable state across
// the two goroutines.
type handlerOutcome struct {
	result any
	err    error
}

type ErrorLogEntry struct {
	TraceID string `json:"trace_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (el *ErrorLogEntry) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s \u001B[38;5;%dm%s \n", el.TraceID, colorCodeError, el.Error)
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := newContext(gofrHTTP.NewResponder(w, r.Method), gofrHTTP.NewRequest(r), h.container)

	traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()

	isWebSocket := websocket.IsWebSocketUpgrade(r)

	if isWebSocket {
		// If the request is a WebSocket upgrade, do not apply the timeout
		c.Context = r.Context()
	} else if h.requestTimeout != 0 {
		ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
		defer cancel()

		c.Context = ctx
	}

	var (
		result any
		err    error
	)

	if !isWebSocket && h.requestTimeout == 0 {
		result, err = h.serveInline(c, traceID)
	} else {
		result, err = h.serveWithGoroutine(c, traceID, r)
	}

	// Handle custom headers if 'result' is a 'Response'.
	if resp, ok := result.(response.Response); ok {
		resp.SetCustomHeaders(w)
	}

	c.responder.Respond(result, err)
}

// serveInline runs the user handler in the calling goroutine. Used when
// there is no server-side request timeout and the request is not a
// WebSocket upgrade — the dominant case in pprof. Avoids the per-request
// goroutine + 2 channels + select that serveWithGoroutine needs (~3-4 KB
// allocations and a goroutine context switch per request).
//
// Cancellation still works: c.Context.Err() is checked after the handler
// returns. The goroutine path's "abort on cancel" was always cosmetic —
// Go can't kill a goroutine, so the handler ran to completion in both
// designs. The client never sees the disconnect-detected response either
// way (TCP connection is gone). Response wire shape is byte-identical to
// serveWithGoroutine because the (result, err) pair feeds the same
// Respond call.
func (h handler) serveInline(c *Context, traceID string) (result any, err error) {
	panicked := false

	func() {
		defer func() {
			if re := recover(); re != nil {
				logPanic(h.container.Logger, re)

				err = gofrHTTP.ErrorPanicRecovery{}
				panicked = true
			}
		}()

		result, err = h.function(c)
	}()

	if !panicked {
		h.logError(traceID, err)
	}

	// Map a canceled / deadline-exceeded ctx to the right error so the
	// wire shape matches serveWithGoroutine:
	//   - context.Canceled         → ErrorClientClosedRequest (HTTP 499)
	//   - context.DeadlineExceeded → ErrorRequestTimeout      (HTTP 408)
	// In both cases we drop the handler's result (which may have been
	// computed despite the cancellation) so Respond emits the bare error
	// envelope instead of a 206 Partial Content.
	if ctxErr := c.Context.Err(); !panicked && ctxErr != nil {
		result = nil
		err = gofrHTTP.ErrorRequestTimeout{}

		if errors.Is(ctxErr, context.Canceled) {
			err = gofrHTTP.ErrorClientClosedRequest{}
		}
	}

	return result, err
}

// serveWithGoroutine runs the user handler in a fresh goroutine and waits
// on a select. Used when h.requestTimeout > 0 (the deadline branch must
// be able to fire while the handler is still running) or when the request
// is a WebSocket upgrade (handleWebSocketUpgrade has to see the request
// after the handler hijacks the connection).
//
// The handler outcome is sent through a buffered channel rather than to
// shared variables, so the goroutine never writes a memory location the
// main goroutine reads — `go test -race` stays clean. Buffer size 1 lets
// the handler goroutine finish writing and exit even after the main
// goroutine has already taken the ctx.Done or panicked branch.
func (h handler) serveWithGoroutine(c *Context, traceID string, r *http.Request) (result any, err error) {
	done := make(chan handlerOutcome, 1)
	panicked := make(chan struct{})

	go func() {
		defer func() {
			panicRecoveryHandler(recover(), h.container.Logger, panicked)
		}()

		res, e := h.function(c)
		h.logError(traceID, e)

		done <- handlerOutcome{res, e}
	}()

	select {
	case <-c.Context.Done():
		// Server-side timeout or client cancellation. Map to the matching
		// gofrHTTP error so Respond emits 408 (timeout) or 499 (client
		// closed).
		err = gofrHTTP.ErrorRequestTimeout{}

		if errors.Is(c.Context.Err(), context.Canceled) {
			err = gofrHTTP.ErrorClientClosedRequest{}
		}
	case out := <-done:
		result = out.result
		err = out.err

		handleWebSocketUpgrade(r)
	case <-panicked:
		err = gofrHTTP.ErrorPanicRecovery{}
	}

	return result, err
}

func healthHandler(c *Context) (any, error) {
	return c.Health(c), nil
}

func liveHandler(*Context) (any, error) {
	return struct {
		Status string `json:"status"`
	}{Status: "UP"}, nil
}

func faviconHandler(*Context) (any, error) {
	data, err := os.ReadFile("./static/favicon.ico")
	if err != nil {
		data, err = static.Files.ReadFile("favicon.ico")
	}

	return response.File{
		Content:     data,
		ContentType: "image/x-icon",
	}, err
}

func catchAllHandler(*Context) (any, error) {
	return nil, gofrHTTP.ErrorInvalidRoute{}
}

func panicRecoveryHandler(re any, log logging.Logger, panicked chan struct{}) {
	if re == nil {
		return
	}

	close(panicked)
	logPanic(log, re)
}

// logPanic emits the panicLog structure used by both serveInline and
// serveWithGoroutine. Single definition prevents the two paths drifting
// in what they record (error message, stack trace) for an otherwise
// identical recover.
func logPanic(log logging.Logger, re any) {
	log.Error(panicLog{
		Error:      fmt.Sprint(re),
		StackTrace: string(debug.Stack()),
	})
}

// Log the error(if any) with traceID and errorMessage.
func (h handler) logError(traceID string, err error) {
	if err != nil {
		errorLog := &ErrorLogEntry{TraceID: traceID, Error: err.Error()}

		// define the default log level for error
		loggerHelper := h.container.Logger.Error

		switch logging.GetLogLevelForError(err) {
		case logging.ERROR:
			// we use the default log level for error
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
}

func handleWebSocketUpgrade(r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		// Do not respond with HTTP headers since this is a WebSocket request
		return
	}
}
