package gofr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/http/response"
)

// defaultHeartbeatInterval is the interval between automatic heartbeat comments.
const defaultHeartbeatInterval = 15 * time.Second

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Name  string // Event type (event: field)
	Data  any    // Event data (data: field) - strings pass through, others are JSON-encoded
	ID    string // Event ID (id: field)
	Retry int    // Reconnection time in milliseconds (retry: field)
}

// SSEStream writes Server-Sent Events directly to the HTTP response.
// It is safe for concurrent use; a mutex serializes all writes.
type SSEStream struct {
	w  http.ResponseWriter
	rc *http.ResponseController
	mu sync.Mutex
}

// SSEFunc is the callback signature for SSE handlers.
// The function receives an SSEStream to write events.
type SSEFunc func(stream *SSEStream) error

// SSE represents a Server-Sent Events response.
// Return this struct from a handler to stream events to the client.
// A heartbeat comment is automatically sent every 15s to keep the connection
// alive through proxies with idle timeouts.
//
// Example:
//
//	app.GET("/events", func(c *gofr.Context) (any, error) {
//	    return gofr.SSE{
//	        Callback: func(stream *gofr.SSEStream) error {
//	            for i := 0; i < 10; i++ {
//	                if err := stream.SendEvent("counter", i); err != nil {
//	                    return err
//	                }
//	                time.Sleep(time.Second)
//	            }
//	            return nil
//	        },
//	    }, nil
//	})
type SSE struct {
	Callback SSEFunc
}

// toResponseSSE converts the user-facing SSE struct into the internal response.SSE
// type understood by the responder. It wraps the callback with SSEStream creation
// and heartbeat management.
func (s SSE) toResponseSSE() response.SSE {
	if s.Callback == nil {
		return response.SSE{Callback: nil}
	}

	cb := s.Callback

	return response.SSE{
		Callback: func(w http.ResponseWriter, rc *http.ResponseController) error {
			stream := &SSEStream{w: w, rc: rc}

			done := make(chan struct{})
			go stream.runHeartbeat(done, defaultHeartbeatInterval)

			defer close(done)

			return cb(stream)
		},
	}
}

// Send writes a formatted SSE event to the stream and flushes.
func (s *SSEStream) Send(event SSEEvent) error {
	return s.writeEvent(event)
}

// SendData is shorthand for Send(SSEEvent{Data: data}).
func (s *SSEStream) SendData(data any) error {
	return s.writeEvent(SSEEvent{Data: data})
}

// SendEvent is shorthand for Send(SSEEvent{Name: name, Data: data}).
func (s *SSEStream) SendEvent(name string, data any) error {
	return s.writeEvent(SSEEvent{Name: name, Data: data})
}

// SendComment writes an SSE comment (: prefix) to the stream.
// Comments are often used as keep-alive heartbeats.
func (s *SSEStream) SendComment(text string) error {
	var sb strings.Builder

	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(&sb, ": %s\n", line)
	}

	sb.WriteString("\n")

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprint(s.w, sb.String()); err != nil {
		return err
	}

	return s.rc.Flush()
}

// writeEvent formats, writes, and flushes a single SSE event.
func (s *SSEStream) writeEvent(event SSEEvent) error {
	raw, err := formatEvent(event)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprint(s.w, raw); err != nil {
		return err
	}

	return s.rc.Flush()
}

// runHeartbeat sends periodic comment frames to keep the connection alive
// through proxies that kill idle connections. Stops when done is closed.
func (s *SSEStream) runHeartbeat(done <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := s.SendComment("heartbeat"); err != nil {
				return
			}
		}
	}
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
