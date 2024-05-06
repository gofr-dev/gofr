package gofr

import (
	"os"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/static"

	"net/http"
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
for now. In the future, this can be considered as well if we are writing our own http router.
*/

type handler struct {
	function  Handler
	container *container.Container
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := newContext(gofrHTTP.NewResponder(w, r.Method), gofrHTTP.NewRequest(r), h.container)
	defer c.Trace("gofr-handler").End()
	c.responder.Respond(h.function(c))
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

		return response.File{
			Content:     data,
			ContentType: "image/x-icon",
		}, err
	}

	return response.File{
		Content:     data,
		ContentType: "image/x-icon",
	}, err
}

func catchAllHandler(*Context) (interface{}, error) {
	return nil, http.ErrMissingFile
}
