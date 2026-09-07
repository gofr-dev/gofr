package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

const (
	defaultHeartbeat  = 15 * time.Second
	streamWriteWindow = 30 * time.Second
)

var (
	errStreamPanic   = errors.New("stream source panicked")
	errNilStream     = errors.New("stream source is nil")
	errStreamCorrupt = errors.New("stream value could not be encoded")
)

// handleStream drains s.Source to the client, flushing after every write. It pulls values on
// demand so a slow client throttles the producer, sends a periodic keep-alive so a dropped client
// is detected while idle, bounds each write with a deadline, and always closes the source — leaving
// no goroutine behind when the client disconnects mid-stream. A Source that also honors its own
// context cancels promptly even without a heartbeat.
func (r Responder) handleStream(s resTypes.Stream) {
	if s.Source == nil {
		r.w.WriteHeader(http.StatusInternalServerError)
		_, _ = r.w.Write([]byte(`{"error":{"message":"` + errNilStream.Error() + `"}}` + "\n"))

		return
	}

	rc := http.NewResponseController(r.w)

	setStreamHeaders(r.w, s.Format)
	r.w.WriteHeader(http.StatusOK)

	_ = rc.Flush()

	defer func() { _ = s.Source.Close() }()

	// items is unbuffered so the producer blocks until the client has taken the last value.
	items := make(chan any)
	quit := make(chan struct{})
	termErr := make(chan error, 1) // the source's terminal error, delivered once at end

	defer close(quit)

	go readStream(s.Source, items, quit, termErr)

	r.pump(rc, s, items, termErr)
}

// readStream pulls from the source into items until the source is done or the consumer quits. A
// panic in the source is recovered into an error frame instead of crashing the server. The
// terminal error is sent before items is closed, so pump reads it safely after draining.
func readStream(src resTypes.Streamer, items chan<- any, quit <-chan struct{}, termErr chan<- error) {
	var err error

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: %v", errStreamPanic, p)
		}

		termErr <- err

		close(items)
	}()

	for {
		v, ok := src.Next()
		if !ok {
			err = src.Err()
			return
		}

		select {
		case items <- v:
		case <-quit:
			return
		}
	}
}

// pump writes values and heartbeats until the stream ends or a write fails (client gone). The
// heartbeat runs for both formats so an idle stream to a dead client is torn down; for NDJSON the
// beat is a flush-only probe that adds no bytes to the wire.
func (r Responder) pump(rc *http.ResponseController, s resTypes.Stream, items <-chan any, termErr <-chan error) {
	interval := s.Heartbeat
	if interval <= 0 {
		interval = defaultHeartbeat
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case v, ok := <-items:
			if !ok {
				r.finishStream(rc, s, <-termErr)
				return
			}

			if r.writeFrame(rc, s.Format, v) != nil {
				return
			}
		case <-ticker.C:
			if r.writeHeartbeat(rc, s.Format) != nil {
				return
			}
		}
	}
}

// finishStream writes the terminal frame — an error frame if the source failed or panicked, else
// the done marker.
func (r Responder) finishStream(rc *http.ResponseController, s resTypes.Stream, readErr error) {
	if readErr != nil {
		_ = r.writeErrorFrame(rc, s.Format, readErr)
		return
	}

	if s.Format == resTypes.SSE {
		_ = r.writeStream(rc, []byte("data: [DONE]\n\n"))
	}
}

func (r Responder) writeFrame(rc *http.ResponseController, format resTypes.Format, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		// An unencodable value corrupts the stream: emit a terminal error frame and stop rather
		// than continue past a gap the client can't interpret.
		_ = r.writeErrorFrame(rc, format, errStreamCorrupt)

		return errStreamCorrupt
	}

	if format == resTypes.NDJSON {
		return r.writeStream(rc, append(payload, '\n'))
	}

	frame := append([]byte("data: "), payload...)
	frame = append(frame, '\n', '\n')

	return r.writeStream(rc, frame)
}

func (r Responder) writeErrorFrame(rc *http.ResponseController, format resTypes.Format, streamErr error) error {
	payload, err := json.Marshal(map[string]string{"error": streamErr.Error()})
	if err != nil {
		return err
	}

	if format == resTypes.NDJSON {
		return r.writeStream(rc, append(payload, '\n'))
	}

	frame := append([]byte("event: error\ndata: "), payload...)
	frame = append(frame, '\n', '\n')

	return r.writeStream(rc, frame)
}

func (r Responder) writeHeartbeat(rc *http.ResponseController, format resTypes.Format) error {
	if format == resTypes.NDJSON {
		// A bare newline is skipped by NDJSON readers but still exercises the write path, so a
		// dead client is detected even while the source is idle.
		return r.writeStream(rc, []byte("\n"))
	}

	return r.writeStream(rc, []byte(": keep-alive\n\n"))
}

// writeStream bounds the write with a deadline so a stalled client cannot block the handler
// indefinitely, then flushes. A failed write means the client is gone; a flush that the writer
// does not support is not a disconnect and is tolerated so buffered delivery still works.
func (r Responder) writeStream(rc *http.ResponseController, b []byte) error {
	_ = rc.SetWriteDeadline(time.Now().Add(streamWriteWindow))

	if _, err := r.w.Write(b); err != nil {
		return err
	}

	if err := rc.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}

	return nil
}

func setStreamHeaders(w http.ResponseWriter, format resTypes.Format) {
	h := w.Header()

	if format == resTypes.NDJSON {
		h.Set("Content-Type", "application/x-ndjson")
	} else {
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
	}
}
