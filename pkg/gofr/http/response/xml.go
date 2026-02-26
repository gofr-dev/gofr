package response

// XML represents a response that should be sent as XML without JSON encoding.
// If ContentType is empty, Responder defaults it to application/xml.
type XML struct {
	Content     []byte
	ContentType string
}
