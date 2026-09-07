package response

import "time"

// Streamer is a generic pull iterator over a sequence of values produced over time, consumed by a
// Stream response to write to the client incrementally. It is a transport-level primitive and
// depends on nothing above this package; higher-level producers (an LLM stream, a log tail) conform
// to it.
//
// Concurrency contract: the responder calls Next on one goroutine and may call Close on another
// while Next is blocked. Close must cause a blocked Next to return (nil, false) promptly;
// implementations must be safe for one concurrent Next and one Close. A Streamer whose Close does
// not unblock Next can leak the producer goroutine when the client disconnects.
type Streamer interface {
	// Next returns the next value and true, or the zero value and false when the stream is done.
	Next() (any, bool)
	// Err returns the terminal error, if any, once Next has returned false.
	Err() error
	// Close releases the stream's resources and unblocks a pending Next.
	Close() error
}

// Format selects the wire encoding of a Stream.
type Format int

const (
	// SSE writes Server-Sent Events ("data: <json>\n\n"), terminated by "data: [DONE]".
	SSE Format = iota
	// NDJSON writes one JSON value per line.
	NDJSON
)

// Stream is a special response type that writes each value from Source to the client incrementally,
// flushing after every write, so a handler can stream tokens, progress or log lines. Values are
// pulled on demand, so a slow client throttles the producer instead of buffering in memory.
//
// The response status is committed to 200 before the first value is produced, so a Source that
// fails immediately still returns 200 followed by an error frame; the error a handler returns
// alongside a Stream is ignored. Values produced after the client disconnects may be dropped.
type Stream struct {
	// Source supplies the values to stream; it is drained and then closed by the responder.
	Source Streamer
	// Format is the wire encoding. The zero value is SSE.
	Format Format
	// Heartbeat is the idle interval at which a keep-alive is sent on an SSE stream so a dropped
	// client is detected while no data flows. Zero selects a default; it is ignored for NDJSON.
	Heartbeat time.Duration
}
