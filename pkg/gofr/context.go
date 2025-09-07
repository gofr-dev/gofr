package gofr

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/cmd/terminal"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/logging"
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

	// Terminal needs to be public as CMD applications need to access various terminal user interface(TUI) features.
	Out terminal.Output

	logging.ContextLogger
}

type AuthInfo interface {
	GetClaims() jwt.MapClaims
	GetUsername() string
	GetAPIKey() string
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

func (c *Context) Bind(i any) error {
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

// WriteMessageToService writes a message to the WebSocket connection associated with the given service name.
// The data parameter can be of type string, []byte, or any struct that can be marshaled to JSON.
func (c *Context) WriteMessageToService(serviceName string, data any) error {
	// Retrieve connection using serviceName
	conn := c.Container.GetWSConnectionByServiceName(serviceName)
	if conn == nil {
		return fmt.Errorf("%w: %s", ErrConnectionNotFound, serviceName)
	}

	message, err := serializeMessage(data)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, message)
}

type authInfo struct {
	claims   jwt.MapClaims
	username string
	apiKey   string
}

// GetAuthInfo is a method on context, to access different methods to retrieve authentication info.
//
// GetAuthInfo().GetClaims() : retrieves the jwt claims.
// GetAuthInfo().GetUsername() : retrieves the username while basic authentication.
// GetAuthInfo().GetAPIKey() : retrieves the APIKey being used for authentication.
func (c *Context) GetAuthInfo() AuthInfo {
	claims, _ := c.Request.Context().Value(middleware.JWTClaim).(jwt.MapClaims)

	APIKey, _ := c.Request.Context().Value(middleware.APIKey).(string)

	username, _ := c.Request.Context().Value(middleware.Username).(string)

	return &authInfo{
		claims:   claims,
		username: username,
		apiKey:   APIKey,
	}
}

// GetClaims returns a response of jwt.MapClaims type when OAuth is enabled.
// It returns nil if called, when OAuth is not enabled.
func (a *authInfo) GetClaims() jwt.MapClaims {
	return a.claims
}

// GetUsername returns the username when basic auth is enabled.
// It returns an empty string if called, when basic auth is not enabled.
func (a *authInfo) GetUsername() string {
	return a.username
}

// GetAPIKey returns the APIKey when APIKey auth is enabled.
// It returns an empty string if called, when APIKey auth is not enabled.
func (a *authInfo) GetAPIKey() string {
	return a.apiKey
}

// func (c *Context) reset(w Responder, r Request) {
//	c.Request = r
//	c.responder = w
//	c.Context = nil
//	// c.Logger = nil // For now, all loggers are same. So, no need to set nil.
// }

func newContext(w Responder, r Request, c *container.Container) *Context {
	return &Context{
		Context:       r.Context(),
		Request:       r,
		responder:     w,
		Container:     c,
		ContextLogger: *logging.NewContextLogger(r.Context(), c.Logger),
	}
}

func newCMDContext(w Responder, r Request, c *container.Container, out terminal.Output) *Context {
	return &Context{
		Context:       r.Context(),
		responder:     w,
		Request:       r,
		Container:     c,
		Out:           out,
		ContextLogger: *logging.NewContextLogger(r.Context(), c.Logger),
	}
}

func (c *Context) GetCorrelationID() string {
	return trace.SpanFromContext(c).SpanContext().TraceID().String()
}
