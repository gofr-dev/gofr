package response

import (
	"net/http"
)

// SSE represents a Server-Sent Events response.
// Return this from a handler to stream events to the client.
type SSE struct {
	// Stream uses the provided ResponseWriter to send Server-Sent Events.
	Stream func(w http.ResponseWriter) error
}
