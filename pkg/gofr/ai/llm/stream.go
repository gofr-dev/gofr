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

var _ ai.Streamer = (*streamer)(nil)

// Stream sends a streaming completion request and returns a lazily-consumed Streamer over the
// server-sent event chunks, each yielding the incremental content string. The response body stays
// open until Close is called. Streaming with tools is not yet supported and returns an error rather
// than silently dropping tool-call deltas.
func (c *Client) Stream(ctx context.Context, messages []ai.Message, opts ...ai.Option) (ai.Streamer, error) {
	if c.svc == nil {
		return nil, errNotConnected
	}

	if len(ai.ApplyOptions(opts...).Tools) > 0 {
		// Streamed tool-call deltas are not assembled yet; reject rather than silently drop them.
		return nil, errStreamToolsUnsup
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
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		return nil, c.statusError(resp.StatusCode, data)
	}

	return newStreamer(resp.Body), nil
}

type lineStatus int

const (
	lineSkip lineStatus = iota
	lineEmit
	lineDone
	lineError
)

type streamer struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	err     error
	done    bool
	usage   ai.Usage
}

func newStreamer(body io.ReadCloser) *streamer {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, streamBufferInit), streamBufferMax)

	return &streamer{body: body, scanner: scanner}
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

	if chunk.Usage != nil {
		s.usage = ai.Usage{PromptTokens: chunk.Usage.PromptTokens, CompletionTokens: chunk.Usage.CompletionTokens}
	}

	if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
		return "", lineSkip
	}

	return chunk.Choices[0].Delta.Content, lineEmit
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
