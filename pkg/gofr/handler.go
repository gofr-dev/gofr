package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/static"
)

type Handler func(c *Context) (interface{}, error)

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
	requestTimeout string
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := newContext(gofrHTTP.NewResponder(w, r.Method), gofrHTTP.NewRequest(r), h.container)

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if websocket.IsWebSocketUpgrade(r) {
		// If the request is a WebSocket upgrade, do not apply the timeout
		ctx = r.Context()
	} else if h.requestTimeout != "" {
		reqTimeout := h.setContextTimeout(h.requestTimeout)

		ctx, cancel = context.WithTimeout(r.Context(), time.Duration(reqTimeout)*time.Second)
		defer cancel()

		c.Context = ctx
	}

	done := make(chan struct{})
	panicked := make(chan struct{})

	var (
		result interface{}
		err    error
	)

	go func() {
		defer panicRecoveryHandler(h.container, panicked)
		// Execute the handler function
		result, err = h.function(c)

		close(done)
	}()

	select {
	case <-c.Context.Done():
		// If the context's deadline has been exceeded, return a timeout error response
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = gofrHTTP.ErrorRequestTimeout{}
		}
	case <-done:
		if websocket.IsWebSocketUpgrade(r) {
			// Do not respond with HTTP headers since this is a WebSocket request
			return
		}
	case <-panicked:
		err = gofrHTTP.ErrorPanicRecovery{}
	}

	// Handler function completed
	c.responder.Respond(result, err)
}

func healthHandler(c *Context) (interface{}, error) {
	return c.Health(c), nil
}

func liveHandler(*Context) (interface{}, error) {
	return struct {
		Status string `json:"status"`
	}{Status: "UP"}, nil
}

func faviconHandler(*Context) (interface{}, error) {
	data, err := os.ReadFile("./static/favicon.ico")
	if err != nil {
		data, err = static.Files.ReadFile("favicon.ico")
	}

	return response.File{
		Content:     data,
		ContentType: "image/x-icon",
	}, err
}

func catchAllHandler(*Context) (interface{}, error) {
	return nil, gofrHTTP.ErrorInvalidRoute{}
}

// Helper function to parse and validate request timeout.
func (h handler) setContextTimeout(timeout string) int {
	reqTimeout, err := strconv.Atoi(timeout)
	if err != nil || reqTimeout < 0 {
		h.container.Error("invalid value of config REQUEST_TIMEOUT. setting default value to 5 seconds.")
	}

	return reqTimeout
}

func panicRecoveryHandler(log logging.Logger, panicked chan struct{}) {
	re := recover()
	if re != nil {
		close(panicked)
		log.Error(panicLog{
			Error:      fmt.Sprint(re),
			StackTrace: string(debug.Stack()),
		})
	}
}
