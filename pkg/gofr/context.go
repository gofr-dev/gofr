package gofr

import (
	ctx "context"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
	"gofr.dev/pkg/log"
)

// Context represents the context information related to an HTTP or command-line (cmd) request within a GoFr application.
//
// It encapsulates essential context data, including the reference to the GoFr application, a logger for recording information,
// a responder for generating responses, a request object for managing incoming requests, and interfaces for managing WebSocket
// connections, server push functionality, and data flushing.
//
// The `Context` type plays a central role in handling and processing requests in a GoFr application, providing access to various
// components and resources needed for request handling, logging, and response generation.
type Context struct {
	// Context is the base context.
	ctx.Context
	// Gofr is a reference to the GoFr application initialization.
	*Gofr
	// Logger is used to log information to an output.
	Logger log.Logger
	resp   responder.Responder
	req    request.Request
	// WebSocketConnection is the connection object for a WebSocket.
	WebSocketConnection *websocket.Conn
	// ServerPush allows a server to pre-emptively send (or "push") responses (along with corresponding "promised" requests)
	// to a client in association with a previous client-initiated request.
	ServerPush http.Pusher
	// ServerFlush is the HTTP server flusher used to flush buffered data to the client.
	ServerFlush http.Flusher
}

// NewContext creates and returns a new Context instance, encapsulating the incoming HTTP request (r), response writer (w),
// and a Gofr object.
// It constructs a context with a specific correlation ID retrieved from the request's header, providing a correlation-aware
// logger instance for logging within the context of the request.
func NewContext(w responder.Responder, r request.Request, g *Gofr) *Context {
	var cID string
	if r != nil {
		cID = r.Header("X-Correlation-ID")
	}

	return &Context{
		req:    r,
		resp:   w,
		Gofr:   g,
		Logger: log.NewCorrelationLogger(cID),
	}
}

func (c *Context) reset(w responder.Responder, r request.Request) {
	c.req = r
	c.resp = w
	c.Context = nil
	c.Logger = nil
}

// Trace returns an open telemetry span. We have to always close the span after corresponding work is done.
func (c *Context) Trace(name string) trace.Span {
	tr := trace.SpanFromContext(c).TracerProvider().Tracer("gofr-context")
	_, span := tr.Start(c.Context, name)

	return span
}

// Request returns the underlying HTTP request
func (c *Context) Request() *http.Request {
	return c.req.Request()
}

// Param retrieves the value of a specified parameter (key) from the associated HTTP request.
// It allows access to route parameters or query parameters, enabling the extraction of specific values from the request's parameters.
func (c *Context) Param(key string) string {
	return c.req.Param(key)
}

// Params retrieves and returns a map containing all the parameters from the associated HTTP request.
// It provides access to all route parameters or query parameters in the form of key-value pairs, allowing comprehensive
// access to request parameters within the context.
func (c *Context) Params() map[string]string {
	return c.req.Params()
}

// PathParam retrieves the value of a specified path parameter (key) from the associated HTTP request.
// It enables access to parameters embedded within the URL path, allowing the extraction of specific values from the
// request's path parameters.
func (c *Context) PathParam(key string) string {
	return c.req.PathParam(key)
}

// Bind binds the incoming data from the HTTP request to a provided interface (i).
// It facilitates the automatic parsing and mapping of request data, such as JSON or form data, into the fields of the provided object.
func (c *Context) Bind(i interface{}) error {
	return c.req.Bind(i)
}

// BindStrict binds the incoming data from the HTTP request to a provided interface (i) while enforcing strict data binding rules.
// It ensures that the request data strictly conforms to the structure of the provided object, returning an error if
// there are any mismatches or missing fields.
func (c *Context) BindStrict(i interface{}) error {
	return c.req.BindStrict(i)
}

// Header retrieves the value of a specified HTTP header (key) from the associated HTTP request.
// It allows access to specific header values sent with the request, enabling retrieval of header information for further
// processing within the context of the request handling.
func (c *Context) Header(key string) string {
	return c.req.Header(key)
}

// Log logs the key-value pair into the logs
func (c *Context) Log(key string, value interface{}) {
	// This section takes care of middleware logging
	if key == "correlationID" { // This condition will not allow the user to unset the CorrelationID.
		return
	}

	r := c.Request()
	appLogData, ok := r.Context().Value(appData).(*sync.Map)

	if !ok {
		c.Logger.Warn("couldn't log appData")
		return
	}

	appLogData.Store(key, value)
	*r = *r.Clone(ctx.WithValue(r.Context(), appData, appLogData))

	// This section takes care of all the individual context loggers
	c.Logger.AddData(key, value)
}

// SetPathParams sets the URL path variables to the given value. These can be accessed
// by c.PathParam(key). This method should only be used for testing purposes.
func (c *Context) SetPathParams(pathParams map[string]string) {
	r := c.req.Request()

	r = mux.SetURLVars(r, pathParams)

	c.req = request.NewHTTPRequest(r)
}

// GetClaims method returns the map of claims
func (c *Context) GetClaims() map[string]interface{} {
	return c.req.GetClaims()
}

// GetClaim method returns the value of claim key provided as the parameter
func (c *Context) GetClaim(claimKey string) interface{} {
	return c.req.GetClaim(claimKey)
}

// ValidateClaimSub validates the "sub" claim within the JWT (JSON Web Token) claims obtained from the request context.
// It compares the provided subject parameter with the "sub" claim value in the JWT claims.
// If the "sub" claim exists and matches the provided subject, the method returns true, indicating a successful validation.
// Otherwise, it returns false, indicating a mismatch or absence of the "sub" claim.
func (c *Context) ValidateClaimSub(subject string) bool {
	claims := c.GetClaims()

	sub, ok := claims["sub"]
	if ok && sub == subject {
		return true
	}

	return false
}

// ValidateClaimsPFCX validates a specific custom claim, "pfcx," within the JWT (JSON Web Token) claims obtained from the request context.
// It compares the provided pfcx parameter with the "pfcx" claim value in the JWT claims.
// If the "pfcx" claim exists and matches the provided pfcx value, the method returns true, indicating a successful validation.
// Otherwise, it returns false, indicating a mismatch or absence of the "pfcx" claim.
func (c *Context) ValidateClaimsPFCX(pfcx string) bool {
	claims := c.GetClaims()

	pfcxValue, ok := claims["pfcx"]
	if ok && pfcxValue == pfcx {
		return true
	}

	return false
}

// ValidateClaimsScope validates a specific scope within the JWT (JSON Web Token) claims obtained from the request context.
// It checks if the provided scope parameter exists within the "scope" claim in the JWT claims.
// If the "scope" claim exists and contains the provided scope value, the method returns true, indicating a successful validation.
// Otherwise, it returns false, indicating a mismatch or absence of the specified scope in the "scope" claim.
func (c *Context) ValidateClaimsScope(scope string) bool {
	claims := c.GetClaims()

	scopes, ok := claims["scope"]

	if !ok {
		return false
	}

	scopesArr := strings.Split(scopes.(string), " ")

	for i := range scopesArr {
		if scopesArr[i] == scope {
			return true
		}
	}

	return false
}

/*
PublishEventWithOptions publishes message to the pubsub(kafka) configured.

# Ability to provide additional options as described in PublishOptions struct

returns error if publish encounters a failure
*/
func (c *Context) PublishEventWithOptions(key string, value interface{}, headers map[string]string, options *pubsub.PublishOptions) error {
	return c.PubSub.PublishEventWithOptions(key, value, headers, options)
}

/*
PublishEvent publishes message to the pubsub(kafka) configured.

	Information like topic is read from config, timestamp is set to current time
	other fields like offset and partition are set to it's default value
	if desire to overwrite these fields, refer PublishEventWithOptions() method above

	returns error if publish encounters a failure
*/
func (c *Context) PublishEvent(key string, value interface{}, headers map[string]string) error {
	return c.PubSub.PublishEvent(key, value, headers)
}

/*
Subscribe read messages from the pubsub(kafka) configured.

	If multiple topics are provided in the environment or
	in kafka config while creating the consumer, reads messages from multiple topics
	reads only one message at a time. If desire to read multiple messages
	call Subscribe in a for loop

	returns error if subscribe encounters a failure
	on success returns the message received in the Message struct format
*/
func (c *Context) Subscribe(target interface{}) (*pubsub.Message, error) {
	message, err := c.PubSub.Subscribe()
	if err != nil {
		return message, err
	}

	return message, c.PubSub.Bind([]byte(message.Value), &target)
}

/*
		SubscribeWithCommit read messages from the pubsub(kafka) configured.

			calls the CommitFunc after subscribing message from kafka and based on
	        the return values decides whether to commit message and consume another message
*/
func (c *Context) SubscribeWithCommit(f pubsub.CommitFunc) (*pubsub.Message, error) {
	return c.PubSub.SubscribeWithCommit(f)
}
