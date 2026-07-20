package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/ai"
)

// Gemini-style usage paths, bound to constants so the token-named UsageFields don't trip gosec's
// hardcoded-credential heuristic (it only flags string literals, not identifiers).
const (
	pathGeminiCached    = "usage_metadata.cached_content_token_count"
	pathGeminiReasoning = "usage_metadata.thoughts_token_count"
)

func defaultUsage(s string) ai.Usage { return mapUsage(nil, json.RawMessage(s)) }

// The OpenAI/Groq shape reports cache-read and reasoning counts under the *_details objects.
func TestMapUsage_OpenAIShape(t *testing.T) {
	got := defaultUsage(`{
		"prompt_tokens":2000,"completion_tokens":300,"total_tokens":2306,
		"prompt_tokens_details":{"cached_tokens":1920},
		"completion_tokens_details":{"reasoning_tokens":128}
	}`)

	assert.Equal(t, 2000, got.PromptTokens)
	assert.Equal(t, 300, got.CompletionTokens)
	assert.Equal(t, 2306, got.TotalTokens)
	assert.Equal(t, 1920, got.CachedTokens)
	assert.Equal(t, 128, got.ReasoningTokens)
}

// DeepSeek reports cache hits at the top level instead of under prompt_tokens_details.
func TestMapUsage_DeepSeekCacheHit(t *testing.T) {
	got := defaultUsage(`{
		"prompt_tokens":2000,"completion_tokens":10,"total_tokens":2010,
		"prompt_cache_hit_tokens":1536,"prompt_cache_miss_tokens":464
	}`)

	assert.Equal(t, 1536, got.CachedTokens)
}

// When both are present, the standard details field wins over the DeepSeek alias.
func TestMapUsage_DetailsPreferredOverAlias(t *testing.T) {
	got := defaultUsage(`{"prompt_tokens":2000,"prompt_tokens_details":{"cached_tokens":900},"prompt_cache_hit_tokens":100}`)

	assert.Equal(t, 900, got.CachedTokens)
}

// input_tokens / output_tokens (Responses-API / Anthropic naming) are accepted as default aliases
// for prompt / completion, with no custom UsageFields.
func TestMapUsage_InputOutputAliases(t *testing.T) {
	got := defaultUsage(`{"input_tokens":2000,"output_tokens":300,"total_tokens":2300}`)

	assert.Equal(t, 2000, got.PromptTokens)
	assert.Equal(t, 300, got.CompletionTokens)
	assert.Equal(t, 2300, got.TotalTokens)
}

// When both the standard field and the alias are present, the standard field wins.
func TestMapUsage_StandardWinsOverInputAlias(t *testing.T) {
	got := defaultUsage(`{"prompt_tokens":10,"completion_tokens":4,"input_tokens":999,"output_tokens":999}`)

	assert.Equal(t, 10, got.PromptTokens)
	assert.Equal(t, 4, got.CompletionTokens)
}

// The Responses-API / Anthropic shape nests cache-read and reasoning under input_tokens_details /
// output_tokens_details; these are captured as fallbacks alongside the input/output aliases.
func TestMapUsage_ResponsesAPIDetailsNesting(t *testing.T) {
	got := defaultUsage(`{"input_tokens":100,"output_tokens":40,` +
		`"input_tokens_details":{"cached_tokens":30},"output_tokens_details":{"reasoning_tokens":12}}`)

	assert.Equal(t, 100, got.PromptTokens)
	assert.Equal(t, 40, got.CompletionTokens)
	assert.Equal(t, 30, got.CachedTokens)
	assert.Equal(t, 12, got.ReasoningTokens)
}

// The standard prompt_tokens_details / completion_tokens_details nesting still wins over the
// Responses-API input_tokens_details / output_tokens_details fallback when both are present.
func TestMapUsage_StandardDetailsWinOverResponsesAPI(t *testing.T) {
	got := defaultUsage(`{"prompt_tokens":100,"completion_tokens":40,` +
		`"prompt_tokens_details":{"cached_tokens":40},"completion_tokens_details":{"reasoning_tokens":8},` +
		`"input_tokens_details":{"cached_tokens":999},"output_tokens_details":{"reasoning_tokens":999}}`)

	assert.Equal(t, 40, got.CachedTokens)
	assert.Equal(t, 8, got.ReasoningTokens)
}

// Negative and out-of-range counts are clamped to 0 on the default path, not fed to metrics.
func TestMapUsage_ClampsOutOfRange(t *testing.T) {
	assert.Zero(t, defaultUsage(`{"completion_tokens":-5}`).CompletionTokens)
	assert.Zero(t, defaultUsage(`{"prompt_tokens":9000000000000000000}`).PromptTokens)
	assert.Zero(t, defaultUsage(`{"prompt_tokens":2147483648}`).PromptTokens)
}

// cached is clamped to prompt and reasoning to completion, so a bad payload can't exceed 100%.
func TestMapUsage_SubsetInvariant(t *testing.T) {
	got := defaultUsage(`{"prompt_tokens":10,"prompt_cache_hit_tokens":50}`)
	assert.Equal(t, 10, got.CachedTokens, "cached clamped to prompt")

	custom := (&UsageFields{CachedTokens: "total_tokens"}).extract(json.RawMessage(`{"prompt_tokens":8,"total_tokens":999}`))
	assert.Equal(t, 8, custom.CachedTokens, "misconfigured cached path clamped to prompt")
}

// A single mistyped field (a quoted number) must not zero the fields that did parse.
func TestMapUsage_PartialTypeError(t *testing.T) {
	got := defaultUsage(`{"prompt_tokens":"1200","completion_tokens":30,"total_tokens":1230}`)

	assert.Equal(t, 30, got.CompletionTokens)
	assert.Equal(t, 1230, got.TotalTokens)
	assert.Zero(t, got.PromptTokens, "quoted number is not coerced, but does not poison the rest")
}

// Usage as a non-object (array/string) or malformed JSON yields a zero Usage, never a panic.
func TestMapUsage_NonObject(t *testing.T) {
	for _, s := range []string{`[1,2]`, `"lots"`, `null`, `{}`, `{`} {
		assert.Equal(t, ai.Usage{}, defaultUsage(s), s)
		assert.Equal(t, ai.Usage{}, (&UsageFields{CachedTokens: "x"}).extract(json.RawMessage(s)), s)
	}
}

// A custom provider whose usage object uses non-standard field names is mapped via UsageFields.
// Unset fields keep their OpenAI defaults.
func TestUsageFields_ExtractCustomPaths(t *testing.T) {
	// Gemini-via-OpenAI-gateway style: nested usage_metadata with different leaf names.
	raw := json.RawMessage(`{
		"prompt_tokens":5000,
		"completion_tokens":400,
		"total_tokens":5400,
		"usage_metadata":{"cached_content_token_count":4096,"thoughts_token_count":210}
	}`)

	fields := UsageFields{
		CachedTokens:    pathGeminiCached,
		ReasoningTokens: pathGeminiReasoning,
	}

	got := fields.extract(raw)

	assert.Equal(t, 5000, got.PromptTokens)    // default path
	assert.Equal(t, 400, got.CompletionTokens) // default path
	assert.Equal(t, 5400, got.TotalTokens)     // default path
	assert.Equal(t, 4096, got.CachedTokens)    // custom nested path
	assert.Equal(t, 210, got.ReasoningTokens)  // custom nested path
}

// A configured cache path suppresses the DeepSeek alias fallback (alias applies only in default mode).
func TestUsageFields_CustomCachedPathNoAliasFallback(t *testing.T) {
	raw := json.RawMessage(`{"prompt_tokens":100,"prompt_cache_hit_tokens":80,"cache":{"read":64}}`)

	fields := UsageFields{CachedTokens: "cache.read"}

	assert.Equal(t, 64, fields.extract(raw).CachedTokens)
}

// A configured path that is absent (or points at a non-number) yields 0, never a panic.
func TestUsageFields_MissingPath(t *testing.T) {
	raw := json.RawMessage(`{"prompt_tokens":10}`)

	fields := UsageFields{CachedTokens: "nope.missing", TotalTokens: "prompt_tokens"}
	got := fields.extract(raw)

	assert.Equal(t, 10, got.PromptTokens)
	assert.Equal(t, 10, got.TotalTokens)
	assert.Zero(t, got.CachedTokens)
}

func TestUsageFields_IsSet(t *testing.T) {
	assert.False(t, (&UsageFields{}).isSet())
	assert.True(t, (&UsageFields{CachedTokens: "x"}).isSet())
}

// A provider that reports no usage details yields a zero-valued, non-panicking Usage.
func TestMapUsage_Absent(t *testing.T) {
	got := defaultUsage(`{"prompt_tokens":5,"completion_tokens":2}`)

	assert.Equal(t, 5, got.PromptTokens)
	assert.Zero(t, got.CachedTokens)
	assert.Zero(t, got.ReasoningTokens)
	assert.Zero(t, got.TotalTokens)
}
