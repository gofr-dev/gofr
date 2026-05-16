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

	// Fast path: no server-side timeout and not a WebSocket upgrade. Run the
	// handler inline with a defer/recover, avoiding the per-request
	// goroutine + 2 channels + select that the timeout path needs. Saves
	// ~3-4 KB allocations and a goroutine context switch per request — the
	// dominant cost in handler.ServeHTTP per pprof.
	//
	// Cancellation detection still works: we read c.Context.Err() after the
	// handler returns. The goroutine path's "abort on cancel" was already
	// only a response-timing concept — Go can't kill goroutines, so the
	// handler ran to completion in both designs. The only observable
	// difference is when the disconnect-detected response is computed
	// (after the handler finishes instead of when the select fires); the
	// client never sees that response either way (TCP connection is gone).
	//
	// Response wire shape is byte-identical to the goroutine path because
	// the resulting (result, err) pair is fed through the same
	// c.responder.Respond.
	if !isWebSocket && h.requestTimeout == 0 {
		panicked := false

		func() {
			defer func() {
				if re := recover(); re != nil {
					h.container.Logger.Error(panicLog{
						Error:      fmt.Sprint(re),
						StackTrace: string(debug.Stack()),
					})

					err = gofrHTTP.ErrorPanicRecovery{}
					panicked = true
				}
			}()

			result, err = h.function(c)
		}()

		if !panicked {
			h.logError(traceID, err)
		}

		// Map a cancelled / deadline-exceeded ctx to the right error so the
		// wire shape matches the goroutine path:
		//   - context.Canceled         → ErrorClientClosedRequest (HTTP 499)
		//   - context.DeadlineExceeded → ErrorRequestTimeout      (HTTP 408)
		// In both cases we drop the handler's result (which may have been
		// computed despite the cancellation) so Respond emits the bare
		// error envelope instead of a 206 Partial Content.
		if ctxErr := c.Context.Err(); !panicked && ctxErr != nil {
			result = nil
			err = gofrHTTP.ErrorRequestTimeout{}

			if errors.Is(ctxErr, context.Canceled) {
				err = gofrHTTP.ErrorClientClosedRequest{}
			}
		}
	} else {
		// Goroutine path: needed when h.requestTimeout > 0 (we have to be
		// able to abandon a handler that exceeds the deadline) or when the
		// request is a WebSocket upgrade (the handler hijacks the
		// connection; Respond is a no-op in that case anyway).
		done := make(chan struct{})
		panicked := make(chan struct{})

		go func() {
			defer func() {
				panicRecoveryHandler(recover(), h.container.Logger, panicked)
			}()
			// Execute the handler function
			result, err = h.function(c)
			h.logError(traceID, err)
			close(done)
		}()

		select {
		case <-c.Context.Done():
			// Handle different context cancellation scenarios
			ctxErr := c.Context.Err()

			// Server-side timeout occurred && fallback for other context errors
			err = gofrHTTP.ErrorRequestTimeout{}

			if errors.Is(ctxErr, context.Canceled) {
				// Client canceled the request (e.g., closed browser tab)
				err = gofrHTTP.ErrorClientClosedRequest{}
			}
		case <-done:
			handleWebSocketUpgrade(r)
		case <-panicked:
			err = gofrHTTP.ErrorPanicRecovery{}
		}
	}

	// Handle custom headers if 'result' is a 'Response'.
	if resp, ok := result.(response.Response); ok {
		resp.SetCustomHeaders(w)
	}

	// Handler function completed
	c.responder.Respond(result, err)
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
