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

### Multiple models

Register more than one model with `gofr.WithName` and select it in a handler with `ctx.LLM(name)`.
A model added without a name is the default, returned by `ctx.LLM()`:

```go
app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b-versatile"})              // default
app.AddLLM(&llm.Client{Provider: llm.OpenAI, Model: "gpt-4o-mini"}, gofr.WithName("fast")) // named

// in a handler
resp, _ := c.LLM().Generate(c, prompt)          // default model
quick, _ := c.LLM("fast").Generate(c, prompt)   // the "fast" model
```

Each model is reported separately on the health endpoint (`llm` for the default, `llm_<name>` for a
named one) and its metrics carry the model's own `provider`/`model` labels.

## Calling the model

`ctx.LLM()` returns the model wrapped with GoFr's instrumentation. It offers:

- `Generate(ctx, prompt, ...opts)` — a convenience over a single-message chat.
- `Chat(ctx, messages, ...opts)` — a multi-turn conversation using `ai.Message` values.
- `Stream(ctx, messages, ...opts)` — an incremental token stream (see below).
- `Embed(ctx, inputs, ...opts)` — text as vectors (see [Embeddings](#embeddings)).
- `Tools()` — the service's own handlers as agent-callable tools (see [Building AI Agents](/docs/advanced-guide/mcp)).

Not every provider supports every method — a chat-only model cannot embed. That is reported by the
call itself, as `ai.ErrEmbedNotSupported` or `ai.ErrStreamNotSupported`, so you handle it as an
ordinary error rather than by checking the model up front.

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

For a chat UI, stream tokens to the client as the model produces them. `ctx.LLM().Stream(...)`
returns a streamer that a handler returns as a [`response.Stream`](/docs/advanced-guide/streaming) —
the same general streaming response GoFr uses for progress and log tailing:

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

See [Streaming Responses](/docs/advanced-guide/streaming) for the SSE and NDJSON formats, heartbeats
and backpressure.

If the model streams tool calls, they are assembled from the provider's deltas and available once the
stream is drained:

```go
stream, _ := ctx.LLM().Stream(ctx, messages, ai.WithTools(tools))
for { v, ok := stream.Next(); if !ok { break }; /* handle content token v */ }

if tc, ok := stream.(ai.ToolCallStreamer); ok {
	for _, call := range tc.ToolCalls() { /* run each assembled tool call */ }
}
```

## Embeddings

Embeddings turn text into vectors — the primitive behind semantic search and agent memory: embed text
on write, embed a query on read, and rank stored vectors by similarity. They ride the same tracing and
token metrics as `Chat`.

Embed is a method on the LLM, like `Chat` and `Stream`:

```go
resp, err := c.LLM("embed").Embed(c, []string{"the quick brown fox", "a fast auburn fox"})
if err != nil {
	return nil, err
}

vectors := resp.Embeddings // one []float32 per input, in order; resp.Usage carries the prompt tokens
```

`vectors[i]` is the embedding of `input[i]`. That is guaranteed, not assumed: the client places each
vector by the `index` the provider reports rather than by its position in the response, so an
OpenAI-compatible backend that returns the array out of order cannot silently pair an input with
someone else's vector. A response that cannot be mapped — a vector count that disagrees with the
inputs sent, an index outside them, or one claimed twice — is returned as an error rather than
half-mapped, so `len(resp.Embeddings) == len(input)` holds whenever `err` is nil.

Embeddings are usually a *different* model from your chat model, so register one with a name and
select it per call:

```go
app.AddLLM(&llm.Client{Provider: llm.OpenAI, Model: "gpt-4o-mini"})                                     // chat (default)
app.AddLLM(&llm.Client{Provider: llm.OpenAI, Model: "text-embedding-3-small"}, gofr.WithName("embed")) // embeddings
```

Not every model can embed — a chat-only model has none. `Embed` reports that as
`ai.ErrEmbedNotSupported`, mirroring how `Stream` returns `ai.ErrStreamNotSupported`, so a
misconfiguration surfaces as an ordinary error. If no model is registered at all, `Embed` returns
`ai.ErrLLMNotConfigured`.

## Limiting concurrency

By default the client sends every request straight to the provider. When many handlers call the model
at once and the provider serializes internally (a single local model, or a tight rate-limit tier),
that burst piles up and tail latency spikes. Set `MaxConcurrentRequests` to cap in-flight calls —
excess `Chat`/`Embed`/`Stream` calls block (honoring their context deadline) until a slot frees:

```go
app.AddLLM(&llm.Client{
	Provider:              llm.Ollama,
	Model:                 "llama3.2:1b",
	MaxConcurrentRequests: 4, // at most 4 requests in flight; 0 (the default) is unlimited
})
```

This is backpressure, not parallelism — it keeps a burst from overwhelming the provider, but the
provider's own throughput (and, for a hosted API, your rate-limit tier) still governs how fast
requests complete.

## Built-in Observability

Every call is observable the same way a normal GoFr request is, joined by the correlation ID.

### Metrics

- **`app_llm_request_count`** — a counter labeled `provider`, `model`, `operation` and `status`.
- **`app_llm_tokens_per_request`** — a histogram labeled `provider`, `model`, `token_type` and
  `status`, where `token_type` is `prompt`, `completion`, `cached` or `reasoning`. `cached` and
  `reasoning` are reported when the provider supports prompt caching or reasoning, and are subsets of
  `prompt` and `completion`. Tokens are recorded whenever the provider reported them — including on a
  failed call that was still billed (a `200`-with-error-object, or a stream that fails mid-drain) —
  under `status="error"`, so failed-but-billed spend stays visible. The Prometheus `_sum` per
  `token_type` is the cumulative token count; a cache-hit rate is
  `sum(cached) / sum(prompt)` (add `status="success"` to either sum to scope it to successful calls).

Metrics are low-cardinality by design — prompts, session IDs and run IDs live on traces, never on
metric labels.

### Traces

A span per call (`llm.chat` / `llm.generate` / `llm.stream` / `llm.embed`) carrying provider, model
and token attributes (`llm.tokens.prompt/completion/total/cached/reasoning`) — a child of the request
span and the parent of the provider's HTTP span. On `llm.embed` only the prompt tokens are non-zero,
since embeddings bill input alone.

### Logs

A structured line per call with provider, model, operation, token counts (including cached and
reasoning when reported) and status. Prompt and completion text are never logged, and the API key
never appears in a log, error, span or health detail.

### Health

The model registers as a datasource, so its reachability is reported on the health endpoint
alongside your databases.

## Configuration

Model name, endpoint and keys come from GoFr's existing config layer (environment / `.env`). No new
mechanism to learn:

```dotenv
GROQ_API_KEY=your-key-here
```

### Custom token-usage fields

Token counts are read from the OpenAI usage shape, which every built-in provider uses, so input
(`prompt_tokens`), output (`completion_tokens`), total, caching and reasoning tokens are captured with
no configuration. The Responses-API / Anthropic names `input_tokens` and `output_tokens` are accepted
as default aliases for prompt and completion, and their `input_tokens_details.cached_tokens` /
`output_tokens_details.reasoning_tokens` nesting is accepted for cached and reasoning; the standard
`prompt_tokens_details` / `completion_tokens_details` shape wins when both are present. For an
OpenAI-compatible provider whose `usage` object names fields differently still, set `UsageFields` — a
dot-separated path per field, where any empty field keeps its default:

```go
app.AddLLM(&llm.Client{
	Provider: llm.OpenAI,
	Model:    "custom-model",
	BaseURL:  "https://my-gateway.example.com/v1",
	UsageFields: llm.UsageFields{
		// only override what differs; prompt/completion/total keep their defaults
		CachedTokens:    "usage_metadata.cached_content_token_count",
		ReasoningTokens: "usage_metadata.thoughts_token_count",
	},
})
```

The mapped counts flow into the same metrics, span attributes and logs as the built-in providers.

> #### Check out the example on how to call an LLM from a handler in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-ai/main.go)
