package gofr

import (
	gofrHTTP "github.com/vikash/gofr/pkg/gofr/http"
	"github.com/vikash/gofr/pkg/gofr/http/response"
	"github.com/vikash/gofr/pkg/gofr/static"

	"net/http"
)

type Handler func(c *Context) (interface{}, error)

/*
Developer Note: There is an implementation where we do not need this internal handler struct
and directly use Handler. However, in that case the container dependency is not injected and
has to be created inside ServeHTTP method, which will result in multiple unnecessary calls.
This is what we implemented first.

There is another possibility where we write out own Router implementation and let httpServer
use that router which will return a Handler and httpServer will then create the context with
injecting container and call that Handler with the new context. A similar implementation is
done in CMD. Since this will require us to write our own router - we are not taking that path
for now. In future, this can be considered as well if we are writing our own http router.
*/

type handler struct {
	function  Handler
	container *Container
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := newContext(gofrHTTP.NewResponder(w), gofrHTTP.NewRequest(r), h.container)
	defer c.Trace("gofr-handler").End()
	c.responder.Respond(h.function(c))
}


func healthHandler(c *Context) (interface{}, error) {
	return "OK", nil
}

func faviconHandler(c *Context) (interface{}, error) {
	data, err := static.Files.ReadFile("favicon.ico")

	return response.File{
		Content:     data,
		ContentType: "image/x-icon",
	}, err
}