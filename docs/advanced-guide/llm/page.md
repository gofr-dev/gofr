---
description: "Call large language models from a GoFr handler with app.AddLLM and ctx.LLM(). Get tracing, token metrics, health checks and streaming for free, with a provider-agnostic, OpenAI-compatible client."
nextjs:
  metadata:
    title: "Calling LLMs in GoFr — Observable, Provider-Agnostic"
    description: "Call large language models from a GoFr handler with app.AddLLM and ctx.LLM(). Get tracing, token metrics, health checks and streaming for free, with a provider-agnostic, OpenAI-compatible client."
---

# Calling LLMs

GoFr lets you talk to a large language model from inside a handler with the same batteries-included
promise as every other datasource: one line to register a model, one call to use it, and tracing,
token metrics, structured logs and health checks happen automatically.

## Registering a model

Add a model with `app.AddLLM`. The bundled `llm.Client` is OpenAI-compatible and works with any
provider that speaks that API. Built-in providers are `llm.OpenAI`, `llm.Groq`, `llm.DeepSeek`,
`llm.Together` and `llm.Ollama`; set `BaseURL` to reach any other OpenAI-compatible endpoint (a local
model, a gateway, an aggregator).

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/ai/llm"
)

func main() {
	app := gofr.New()

	// The API key is read from <PROVIDER>_API_KEY (here GROQ_API_KEY) in the environment.
	app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b-versatile"})

	app.POST("/summarize", summarize)

	app.Run()
}

func summarize(c *gofr.Context) (any, error) {
	var in struct {
		Text string `json:"text"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	return c.LLM().Generate(c, "Summarize the following:\n"+in.Text)
}
```

Switching provider is a one-field change — `Provider: llm.OpenAI` and the matching
`OPENAI_API_KEY` — because every provider satisfies the same `ai.Model` interface. Set `BaseURL` to
point at any other OpenAI-compatible endpoint.

### Zero-config from the environment

Like Redis and SQL, an OpenAI-compatible model is wired automatically when its configuration is
present — no `AddLLM` call needed:

```dotenv
LLM_PROVIDER=groq                    # openai | groq | deepseek | together | ollama
LLM_MODEL=llama-3.3-70b-versatile
LLM_API_KEY=your-key-here            # generic key, used for whichever provider is selected
# LLM_BASE_URL=...                   # optional override for any OpenAI-compatible endpoint
```

The key is read from the generic `LLM_API_KEY` first, then the provider-specific variable
(`OPENAI_API_KEY`, `GROQ_API_KEY`, `DEEPSEEK_API_KEY`, `TOGETHER_API_KEY`, `OLLAMA_API_KEY`) — so an
existing `OPENAI_API_KEY` in your environment is honored without renaming.

`app.AddLLM` stays available for a custom `ai.Model`, programmatic configuration, or to override the
model wired from the environment.

## Calling the model

`ctx.LLM()` returns the model wrapped with GoFr's instrumentation. It offers:

- `Generate(ctx, prompt, ...opts)` — a convenience over a single-message chat.
- `Chat(ctx, messages, ...opts)` — a multi-turn conversation using `ai.Message` values.
- `Stream(ctx, messages, ...opts)` — an incremental token stream (see below).
- `Tools()` — the service's own handlers as agent-callable tools (see [Building AI Agents](/docs/advanced-guide/mcp)).

Options are applied per call: `ai.WithTemperature(0.2)`, `ai.WithMaxTokens(512)`, `ai.WithTools(...)`.

```go
resp, err := c.LLM().Chat(c, []ai.Message{
	{Role: ai.RoleSystem, Content: "You are a terse assistant."},
	{Role: ai.RoleUser, Content: "Name three primary colors."},
}, ai.WithTemperature(0.2))
```

The returned `*ai.Response` carries `Content`, `ToolCalls`, `Usage` (prompt and completion tokens)
and the resolved `Model`.

## Streaming responses

For a chat UI, stream tokens to the client as the model produces them by returning a
`response.Stream`. GoFr writes them as server-sent events, flushing after each and applying
backpressure so a slow client throttles the producer instead of buffering in memory.

```go
func chat(c *gofr.Context) (any, error) {
	var in struct {
		Prompt string `json:"prompt"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	stream, err := c.LLM().Stream(c, []ai.Message{{Role: ai.RoleUser, Content: in.Prompt}})
	if err != nil {
		return nil, err
	}

	return response.Stream{Source: stream}, nil
}
```

`response.Stream` also accepts `Format: response.NDJSON` and a `Heartbeat` interval, and works for
any streaming source — progress updates or log tailing — not just LLMs.

If the model streams tool calls, they are assembled from the provider's deltas and available once the
stream is drained:

```go
stream, _ := ctx.LLM().Stream(ctx, messages, ai.WithTools(tools))
for { v, ok := stream.Next(); if !ok { break }; /* handle content token v */ }

if tc, ok := stream.(ai.ToolCallStreamer); ok {
	for _, call := range tc.ToolCalls() { /* run each assembled tool call */ }
}
```

## What you get for free

Every call is observable the same way a normal GoFr request is, joined by the correlation ID:

| Signal | Detail |
|---|---|
| Metrics | `app_llm_request_count` (provider, model, operation, status) and `app_llm_tokens_per_request` histogram (provider, model, token_type). The histogram's Prometheus `_sum` is the cumulative token count. |
| Traces | A span per call (`llm.chat` / `llm.generate` / `llm.stream`) with provider, model and token attributes — child of the request span, parent of the provider's HTTP span. |
| Logs | A structured line per call with provider, model, operation, tokens and status. Prompt and completion text are never logged. |
| Health | The model registers as a datasource, so its reachability is reported on the health endpoint alongside your databases. |

Metrics are low-cardinality by design — prompts, session IDs and run IDs live on traces, never on
metric labels. The API key never appears in a log, error, span or health detail.

## Configuration

Model name, endpoint and keys come from GoFr's existing config layer (environment / `.env`). No new
mechanism to learn:

```dotenv
GROQ_API_KEY=your-key-here
```
