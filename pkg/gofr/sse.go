package gofr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gofr.dev/pkg/gofr/http/response"
)

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Name  string // Event type (event: field)
	Data  any    // Event data (data: field) - strings pass through, others are JSON-encoded
	ID    string // Event ID (id: field)
	Retry int    // Reconnection time in milliseconds (retry: field)
}

// SSEStream writes Server-Sent Events directly to the HTTP response.
// It wraps an http.ResponseWriter and flushes after each event.
type SSEStream struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

// SSEFunc is the callback signature for SSE handlers.
// The function receives an SSEStream to write events.
type SSEFunc func(stream *SSEStream) error

// SSEResponse creates an SSE response that can be returned from a handler.
// The callback function is called by the Responder to produce SSE events.
//
// Example:
//
//	app.GET("/events", func(c *gofr.Context) (any, error) {
//	    return gofr.SSEResponse(func(stream *gofr.SSEStream) error {
//	        for i := 0; i < 10; i++ {
//	            if err := stream.SendEvent("counter", i); err != nil {
//	                return err
//	            }
//	            time.Sleep(time.Second)
//	        }
//	        return nil
//	    }), nil
//	})
func SSEResponse(callback SSEFunc) response.SSE {
	return response.SSE{
		Stream: func(w http.ResponseWriter) error {
			stream := &SSEStream{
				w:  w,
				rc: http.NewResponseController(w),
			}
			return callback(stream)
		},
	}
}

// Send writes a formatted SSE event to the stream and flushes.
func (s *SSEStream) Send(event any) error {
	sseEvent, ok := event.(SSEEvent)
	if !ok {
		// If not an SSEEvent, treat as data-only event
		return s.SendData(event)
	}

	raw, err := formatEvent(sseEvent)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(s.w, raw); err != nil {
		return err
	}

	return s.rc.Flush()
}

// SendData is shorthand for Send(SSEEvent{Data: data}).
func (s *SSEStream) SendData(data any) error {
	return s.Send(SSEEvent{Data: data})
}

// SendEvent is shorthand for Send(SSEEvent{Name: name, Data: data}).
func (s *SSEStream) SendEvent(name string, data any) error {
	return s.Send(SSEEvent{Name: name, Data: data})
}

// SendComment writes an SSE comment (: prefix) to the stream.
// Comments are often used as keep-alive heartbeats.
func (s *SSEStream) SendComment(text string) error {
	var sb strings.Builder

	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(&sb, ": %s\n", line)
	}

	sb.WriteString("\n")

	if _, err := fmt.Fprint(s.w, sb.String()); err != nil {
		return err
	}

	return s.rc.Flush()
}

// formatEvent builds the wire-format string for one SSE event.
func formatEvent(event SSEEvent) (string, error) {
	var sb strings.Builder

	if event.ID != "" {
		fmt.Fprintf(&sb, "id: %s\n", event.ID)
	}

	if event.Name != "" {
		fmt.Fprintf(&sb, "event: %s\n", event.Name)
	}

	if event.Retry > 0 {
		fmt.Fprintf(&sb, "retry: %d\n", event.Retry)
	}

	dataStr, err := formatSSEData(event.Data)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(dataStr, "\n") {
		fmt.Fprintf(&sb, "data: %s\n", line)
	}

	sb.WriteString("\n")

	return sb.String(), nil
}

// formatSSEData converts data to a string for SSE.
// Strings and []byte pass through; everything else is JSON-encoded.
func formatSSEData(data any) (string, error) {
	switch v := data.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal SSE data: %w", err)
		}

		return string(b), nil
	}
}
