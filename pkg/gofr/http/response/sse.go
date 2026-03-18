package response

// SSE represents a Server-Sent Events response.
// Return this from a handler to stream events to the client.
type SSE struct {
	// Callback holds the user's SSE streaming function.
	// Typed as any to avoid circular imports; the Responder type-asserts it at call-site.
	Callback any
}
