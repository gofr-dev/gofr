package gofr

// Responder is used by the application to provide output. This is implemented for both
// cmd and HTTP server application.
type Responder interface {
	Respond(data any, err error)
}

// noopResponder is used by GraphQL resolvers. GraphQL reuses *gofr.Context (which
// requires a Responder) but handles its own response serialization — the resolver
// result is collected by the GraphQL engine and written as part of the unified
// GraphQL JSON response, not via the standard HTTP responder.
type noopResponder struct{}

func (noopResponder) Respond(_ any, _ error) {}
