package gofr

import (
	"context"

	"github.com/gorilla/websocket"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/container"
)

type Context struct {
	context.Context

	// Request needs to be public because handlers need to access request details. Else, we need to provide all
	// functionalities of the Request as a method on context. This is not needed because Request here is an interface
	// So, internals are not exposed anyway.
	Request

	// Same logic as above.
	*container.Container

	// responder is private as Handlers do not need to worry about how to respond. But it is still an abstraction over
	// normal response writer as we want to keep the context independent of http. Will help us in writing CMD application
	// or gRPC servers etc using the same handler signature.
	responder Responder
}

/*
Trace returns an open telemetry span. We have to always close the span after corresponding work is done. Usages:

	span := c.Trace("Some Work")
	// Do some work here.
	defer span.End()

If an entire function has to traced as span, we can use a simpler format:

	defer c.Trace("ExampleHandler").End()

We can write this at the start of function and because of how defer works, trace will start at that line
but End will be called after function ends.

Developer Note: If you chain methods in a defer statement, everything except the last function will be evaluated at call time.
*/
func (c *Context) Trace(name string) trace.Span {
	tr := otel.GetTracerProvider().Tracer("gofr-context")
	ctx, span := tr.Start(c.Context, name)
	// TODO: If we don't close the span using `defer` and run the http-server example by hitting `/trace` endpoint, we are
	// getting incomplete redis spans when viewing the trace using correlationID. If we remove assigning the ctx to GoFr
	// context then spans are coming correct but then parent-child span relationship is being hindered.

	c.Context = ctx

	return span
}

func (c *Context) Bind(i interface{}) error {
	return c.Request.Bind(i)
}

// WriteMessageToSocket writes a message to the WebSocket connection associated with the context.
// The data parameter can be of type string, []byte, or any struct that can be marshaled to JSON.
// It retrieves the WebSocket connection from the context and sends the message as a TextMessage.
func (c *Context) WriteMessageToSocket(data any) error {
	// Retrieve connection from context based on connectionID
	conn := c.Container.GetConnectionFromContext(c.Context)

	message, err := serializeMessage(data)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, message)
}

// func (c *Context) reset(w Responder, r Request) {
//	c.Request = r
//	c.responder = w
//	c.Context = nil
//	// c.Logger = nil // For now, all loggers are same. So, no need to set nil.
// }

func newContext(w Responder, r Request, c *container.Container) *Context {
	return &Context{
		Context:   r.Context(),
		Request:   r,
		responder: w,
		Container: c,
	}
}
