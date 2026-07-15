package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OpenAI/Groq shape reports cache-read and reasoning counts under the *_details objects.
func TestWireUsage_ToAI_OpenAIShape(t *testing.T) {
	var u wireUsage
	require.NoError(t, json.Unmarshal([]byte(`{
		"prompt_tokens":2000,"completion_tokens":300,"total_tokens":2306,
		"prompt_tokens_details":{"cached_tokens":1920},
		"completion_tokens_details":{"reasoning_tokens":128}
	}`), &u))

	got := u.toAI()

	assert.Equal(t, 2000, got.PromptTokens)
	assert.Equal(t, 300, got.CompletionTokens)
	assert.Equal(t, 2306, got.TotalTokens)
	assert.Equal(t, 1920, got.CachedTokens)
	assert.Equal(t, 128, got.ReasoningTokens)
}

// DeepSeek reports cache hits at the top level instead of under prompt_tokens_details.
func TestWireUsage_ToAI_DeepSeekCacheHit(t *testing.T) {
	var u wireUsage
	require.NoError(t, json.Unmarshal([]byte(`{
		"prompt_tokens":2000,"completion_tokens":10,"total_tokens":2010,
		"prompt_cache_hit_tokens":1536,"prompt_cache_miss_tokens":464
	}`), &u))

	assert.Equal(t, 1536, u.toAI().CachedTokens)
}

// When both are present, the standard details field wins over the DeepSeek alias.
func TestWireUsage_ToAI_DetailsPreferredOverAlias(t *testing.T) {
	u := wireUsage{PromptCacheHitTokens: 100}
	u.PromptTokensDetails.CachedTokens = 900

	assert.Equal(t, 900, u.toAI().CachedTokens)
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
		CachedTokens:    "usage_metadata.cached_content_token_count",
		ReasoningTokens: "usage_metadata.thoughts_token_count",
	}

	got := fields.extract(raw)

	assert.Equal(t, 5000, got.PromptTokens)   // default path
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
	assert.False(t, UsageFields{}.isSet())
	assert.True(t, UsageFields{CachedTokens: "x"}.isSet())
}

// A provider that reports no usage details yields a zero-valued, non-panicking Usage.
func TestWireUsage_ToAI_Absent(t *testing.T) {
	var u wireUsage
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_tokens":5,"completion_tokens":2}`), &u))

	got := u.toAI()

	assert.Equal(t, 5, got.PromptTokens)
	assert.Zero(t, got.CachedTokens)
	assert.Zero(t, got.ReasoningTokens)
	assert.Zero(t, got.TotalTokens)
}
