package gofr

// Responder is used by the application to provide output. This is implemented for both
// cmd and HTTP server application.
type Responder interface {
	Respond(data any, err error)
}

type noopResponder struct{}

func (noopResponder) Respond(_ any, _ error) {}
