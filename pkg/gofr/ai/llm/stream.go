package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gofr.dev/pkg/gofr/ai"
)

const (
	streamBufferInit = 64 * 1024
	streamBufferMax  = 1024 * 1024

	dataPrefix = "data:"
	doneMarker = "[DONE]"
)

var (
	_ ai.Streamer         = (*streamer)(nil)
	_ ai.ToolCallStreamer = (*streamer)(nil)
)

// Stream sends a streaming completion request and returns a lazily-consumed Streamer over the
// server-sent event chunks, each Next yielding the incremental content string. Tool calls streamed
// as deltas are assembled and available via ToolCalls once the stream is drained. The response body
// stays open until Close is called.
func (c *Client) Stream(ctx context.Context, messages []ai.Message, opts ...ai.Option) (ai.Streamer, error) {
	if c.svc == nil {
		return nil, errNotConnected
	}

	body, err := c.buildRequest(messages, opts, true)
	if err != nil {
		return nil, err
	}

	resp, err := c.post(ctx, chatCompletionsPath, body)
	if err != nil {
		return nil, err
	}

	if !isSuccess(resp.StatusCode) {
		// Bound the error body: statusError truncates for the message, but read a capped amount so a
		// misbehaving endpoint streaming an endless error body cannot exhaust memory.
		data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyLen+1))
		_ = resp.Body.Close()

		return nil, c.statusError(resp.StatusCode, data)
	}

	return newStreamer(resp.Body, &c.UsageFields), nil
}

type lineStatus int

const (
	lineSkip lineStatus = iota
	lineEmit
	lineDone
	lineError
)

type streamer struct {
	body        io.ReadCloser
	scanner     *bufio.Scanner
	err         error
	done        bool
	usage       ai.Usage
	usageFields *UsageFields

	// tool calls are assembled from deltas keyed by index; toolOrder preserves first-seen order.
	toolAcc   map[int]*ai.ToolCall
	toolOrder []int
}

func newStreamer(body io.ReadCloser, fields *UsageFields) *streamer {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, streamBufferInit), streamBufferMax)

	return &streamer{body: body, scanner: scanner, usageFields: fields, toolAcc: make(map[int]*ai.ToolCall)}
}

// Next pulls the next incremental content delta. It returns the delta string and true, or nil and
// false once the stream ends or an error is set.
func (s *streamer) Next() (any, bool) {
	if s.done || s.err != nil {
		return nil, false
	}

	for s.scanner.Scan() {
		content, status := s.handleLine(s.scanner.Text())

		switch status {
		case lineEmit:
			return content, true
		case lineDone:
			s.done = true

			return nil, false
		case lineError:
			return nil, false
		case lineSkip:
			continue
		}
	}

	s.finish()

	return nil, false
}

func (s *streamer) handleLine(raw string) (string, lineStatus) {
	data, ok := strings.CutPrefix(strings.TrimSpace(raw), dataPrefix)
	if !ok {
		return "", lineSkip
	}

	data = strings.TrimSpace(data)
	if data == doneMarker {
		return "", lineDone
	}

	var chunk streamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		s.err = fmt.Errorf("%w: %w", errStreamRead, err)

		return "", lineError
	}

	if chunk.Error != nil {
		s.err = fmt.Errorf("%w: %s", errProvider, chunk.Error.Message)

		return "", lineError
	}

	// A usage-bearing chunk updates usage; mapUsage ignores an absent or JSON-null usage so a
	// trailing keep-alive chunk cannot wipe a value captured on the finish chunk.
	if u := mapUsage(s.usageFields, chunk.Usage); u != (ai.Usage{}) {
		s.usage = u
	}

	if len(chunk.Choices) == 0 {
		return "", lineSkip
	}

	s.accumulateToolCalls(chunk.Choices[0].Delta.ToolCalls)

	if chunk.Choices[0].Delta.Content == "" {
		return "", lineSkip
	}

	return chunk.Choices[0].Delta.Content, lineEmit
}

// accumulateToolCalls merges a chunk's tool-call deltas into the per-index accumulator: id and name
// arrive once, arguments are concatenated fragment by fragment.
func (s *streamer) accumulateToolCalls(deltas []streamToolCallDelta) {
	for i := range deltas {
		d := &deltas[i]

		acc := s.toolAcc[d.Index]
		if acc == nil {
			acc = &ai.ToolCall{}
			s.toolAcc[d.Index] = acc
			s.toolOrder = append(s.toolOrder, d.Index)
		}

		if d.ID != "" {
			acc.ID = d.ID
		}

		if d.Function.Name != "" {
			acc.Name = d.Function.Name
		}

		acc.Args = append(acc.Args, d.Function.Arguments...)
	}
}

// ToolCalls returns the tool calls assembled during the stream, or nil if none. Empty arguments are
// normalized to an empty JSON object so the result is always valid JSON.
func (s *streamer) ToolCalls() []ai.ToolCall {
	if len(s.toolOrder) == 0 {
		return nil
	}

	out := make([]ai.ToolCall, 0, len(s.toolOrder))

	for _, idx := range s.toolOrder {
		tc := s.toolAcc[idx]

		// Keep Args valid JSON: empty (zero-arg tools) and any truncated/invalid assembly from a
		// broken stream both normalize to an empty object rather than emitting invalid JSON.
		args := tc.Args
		if len(args) == 0 || !json.Valid(args) {
			args = json.RawMessage("{}")
		}

		out = append(out, ai.ToolCall{ID: tc.ID, Name: tc.Name, Args: args})
	}

	return out
}

func (s *streamer) finish() {
	if err := s.scanner.Err(); err != nil {
		s.err = fmt.Errorf("%w: %w", errStreamRead, err)

		return
	}

	s.done = true
}

// Err returns the first error encountered while reading the stream.
func (s *streamer) Err() error { return s.err }

// Close closes the underlying response body.
func (s *streamer) Close() error { return s.body.Close() }

// Usage returns token usage reported by the final chunk, or the zero value if none was sent.
func (s *streamer) Usage() ai.Usage { return s.usage }
