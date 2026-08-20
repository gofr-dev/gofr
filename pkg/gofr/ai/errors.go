package ai

import "errors"

var (
	// ErrStreamNotSupported is returned by Stream when the underlying model cannot stream.
	ErrStreamNotSupported = errors.New("streaming is not supported by this model")
	// ErrEmbedNotSupported is returned by Embed when the underlying model cannot embed.
	ErrEmbedNotSupported = errors.New("embeddings are not supported by this model")
	// ErrToolNotFound is returned by Tools.Call for an unknown tool name.
	ErrToolNotFound = errors.New("tool not found")
	// ErrLLMNotConfigured is returned by every call on the LLM from ctx.LLM(name) when no model is
	// registered under that name (or none at all). ctx.LLM(name) returns this error-yielding LLM
	// rather than a nil interface, so a typo'd or absent model name gives a clear error instead of a
	// nil-pointer panic.
	ErrLLMNotConfigured = errors.New("llm not configured")
)
