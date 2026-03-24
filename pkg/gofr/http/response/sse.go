package response

import "net/http"

// SSECallback is the function signature for SSE streaming callbacks.
type SSECallback func(w http.ResponseWriter, rc *http.ResponseController) error

// SSE represents a Server-Sent Events response.
// Return this from a handler to stream events to the client.
type SSE struct {
	Callback SSECallback
}
