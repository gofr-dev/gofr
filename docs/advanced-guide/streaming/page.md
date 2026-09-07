---
description: "Stream data to clients incrementally with response.Stream — Server-Sent Events or NDJSON, with backpressure, heartbeats and leak-free client-disconnect handling. Works for progress updates, log tailing or LLM tokens."
nextjs:
  metadata:
    title: "Streaming Responses in GoFr — SSE and NDJSON"
    description: "Stream data to clients incrementally with response.Stream — Server-Sent Events or NDJSON, with backpressure, heartbeats and leak-free client-disconnect handling. Works for progress updates, log tailing or LLM tokens."
---

# Streaming Responses

Most handlers return a single value, which GoFr sends when the handler returns. But some responses
are produced over time — a long job's progress, a tail of log lines, or tokens from a language model.
For those, GoFr gives you `response.Stream`: return it from a handler and GoFr writes each value to
the client the moment it is produced, instead of buffering the whole response in memory.

`response.Stream` is a transport primitive, not tied to any one producer — the same type carries
progress events, log lines and LLM tokens.

## Returning a stream

A handler returns `response.Stream` with a `Source` that implements `response.Streamer` — a pull
iterator GoFr drains one value at a time:

```go
type Streamer interface {
	Next() (any, bool) // the next value and true, or the zero value and false when done
	Err() error        // the terminal error, if any, once Next has returned false
	Close() error      // release resources and unblock a pending Next
}
```

For example, a handler that streams a job's progress to the client:

```go
// jobProgress emits 25, 50, 75, 100 and then ends.
type jobProgress struct{ pct int }

func (p *jobProgress) Next() (any, bool) {
	if p.pct >= 100 {
		return nil, false
	}

	p.pct += 25

	return map[string]any{"progress": p.pct}, true
}

func (p *jobProgress) Err() error   { return nil }
func (p *jobProgress) Close() error { return nil }

func trackJob(c *gofr.Context) (any, error) {
	return response.Stream{Source: &jobProgress{}}, nil
}
```

Each value is JSON-encoded and flushed immediately, so the client receives `{"progress":25}`,
`{"progress":50}`, … as they are produced rather than all at once at the end.

## Formats

`Format` selects the wire encoding; the zero value is Server-Sent Events.

```go
return response.Stream{Source: src, Format: response.NDJSON}, nil
```

- **`response.SSE`** (default) — Server-Sent Events: `data: <json>\n\n` per value, terminated by
  `data: [DONE]`. Best for browsers (`EventSource`).
- **`response.NDJSON`** — one JSON value per line. Best for programmatic clients that read a stream
  line by line.

## Backpressure, heartbeats and disconnects

- **Backpressure** — values are pulled on demand over an unbuffered channel, so a slow client
  throttles the producer instead of the server buffering the whole stream in memory.
- **Heartbeats** — on an SSE stream, a keep-alive is sent at an idle interval so a dropped client is
  detected even while no data flows. Set the interval with `Heartbeat`; the zero value picks a
  default.
- **Client disconnect** — when the client goes away, GoFr tears the stream down and calls your
  `Source.Close()`. Your `Close` must unblock a pending `Next` (for example, by closing the channel
  it reads from); otherwise the producer goroutine can leak.

The response status is committed to `200 OK` before the first value is produced, so a source that
fails immediately still returns `200` followed by an error frame — the error a handler returns
alongside a `Stream` is ignored.

## Streaming LLM tokens

Streaming a language model's output is one use of this same type: `ctx.LLM().Stream(...)` returns a
`response.Streamer`, so a handler streams tokens by returning it as the `Source`. See
[Calling LLMs](/docs/advanced-guide/llm) for the AI-specific details.

> #### Check out the example on how to stream a response to the client in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-ai/main.go)
