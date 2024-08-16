package gofr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
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
	requestTimeout time.Duration
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := newContext(gofrHTTP.NewResponder(w, r.Method), gofrHTTP.NewRequest(r), h.container)

	if websocket.IsWebSocketUpgrade(r) {
		// If the request is a WebSocket upgrade, do not apply the timeout
		c.Context = r.Context()
	} else if h.requestTimeout != 0 {
		ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
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
		defer func() {
			panicRecoveryHandler(recover(), h.container, panicked)
		}()
		// Execute the handler function
		result, err = h.function(c)

		close(done)
	}()

	select {
	case <-c.Context.Done():
		// If the context's deadline has been exceeded, return a timeout error response
		if errors.Is(c.Err(), context.DeadlineExceeded) {
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
