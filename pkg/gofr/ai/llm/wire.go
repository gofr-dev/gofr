package llm

import (
	"encoding/json"
	"math"
	"strings"

	"gofr.dev/pkg/gofr/ai"
)

const functionType = "function"

type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []wireMessage  `json:"messages"`
	Temperature   *float64       `json:"temperature,omitempty"`
	MaxTokens     *int           `json:"max_tokens,omitempty"`
	Tools         []wireTool     `json:"tools,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

// streamOptions requests a final usage chunk on a streaming response. OpenAI, Together and Ollama
// only emit stream usage when include_usage is set; providers that ignore it are unaffected.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []wireRequestTool `json:"tool_calls,omitempty"`
}

type wireRequestTool struct {
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type"`
	Function wireRequestFunc `json:"function"`
}

type wireRequestFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFuncSpec `json:"function"`
}

type wireFuncSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type chatResponse struct {
	Model   string          `json:"model"`
	Choices []wireChoice    `json:"choices"`
	Usage   json.RawMessage `json:"usage"`
	Error   *wireError      `json:"error"`
}

// wireError is the error object OpenAI-compatible providers return, sometimes with HTTP 200.
type wireError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type wireChoice struct {
	Message wireRespMessage `json:"message"`
}

type wireRespMessage struct {
	Content   string             `json:"content"`
	ToolCalls []wireResponseTool `json:"tool_calls"`
}

type wireResponseTool struct {
	ID       string           `json:"id"`
	Function wireResponseFunc `json:"function"`
}

type wireResponseFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Default JSON paths for token usage, matching the OpenAI Chat Completions shape used by every
// built-in provider (OpenAI, Groq, DeepSeek, Together, Ollama). Cache-read and reasoning counts live
// under the *_details objects; DeepSeek instead reports cache hits at the top level. A custom
// provider overrides only the fields whose names differ.
const (
	pathPromptTokens     = "prompt_tokens"
	pathCompletionTokens = "completion_tokens"
	pathTotalTokens      = "total_tokens"
	pathCachedTokens     = "prompt_tokens_details.cached_tokens"
	pathReasoningTokens  = "completion_tokens_details.reasoning_tokens"
	pathDeepSeekCached   = "prompt_cache_hit_tokens"
)

// UsageFields remaps the JSON paths GoFr reads token counts from, for OpenAI-compatible providers
// whose usage object deviates from the standard shape. Each field is a dot-separated path into the
// response's usage object (e.g. "usage_metadata.cached_content_token_count"); a key that literally
// contains a dot is not addressable. Every empty field keeps its built-in default, so a custom
// provider sets only what differs and the popular providers need no configuration at all.
type UsageFields struct {
	PromptTokens     string
	CompletionTokens string
	TotalTokens      string
	CachedTokens     string
	ReasoningTokens  string
}

func (f UsageFields) isSet() bool { return f != UsageFields{} }

// extract reads token counts from a raw usage object by path, falling back to the OpenAI defaults for
// any unset field. Each count is clamped to a sane range by intAtPath, and cached/reasoning are
// clamped to their supersets (prompt/completion) so a malformed or misconfigured payload cannot make
// a cache-hit rate exceed 100% or a billable-token count go negative.
func (f UsageFields) extract(rawUsage json.RawMessage) ai.Usage {
	if len(rawUsage) == 0 {
		return ai.Usage{}
	}

	var m map[string]any
	if err := json.Unmarshal(rawUsage, &m); err != nil {
		return ai.Usage{}
	}

	at := func(configured, def string) int {
		if configured != "" {
			return intAtPath(m, configured)
		}

		return intAtPath(m, def)
	}

	prompt := at(f.PromptTokens, pathPromptTokens)
	completion := at(f.CompletionTokens, pathCompletionTokens)

	cached := at(f.CachedTokens, pathCachedTokens)
	if cached == 0 && f.CachedTokens == "" {
		cached = intAtPath(m, pathDeepSeekCached) // DeepSeek alias, only in default mode
	}

	return ai.Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      at(f.TotalTokens, pathTotalTokens),
		CachedTokens:     min(cached, prompt),          // cached is a subset of prompt
		ReasoningTokens:  min(at(f.ReasoningTokens, pathReasoningTokens), completion), // subset of completion
	}
}

// mapUsage turns a raw usage object into ai.Usage using the custom UsageFields paths when configured
// or the built-in OpenAI mapping otherwise (an empty UsageFields resolves to the default paths). An
// absent or JSON-null usage yields the zero value, so a trailing "usage": null chunk never overwrites
// a previously captured usage. Both paths go through the same map-based, field-tolerant, clamped
// extraction, so one mistyped field can never zero out the counts that did parse.
func mapUsage(fields UsageFields, raw json.RawMessage) ai.Usage {
	if len(raw) == 0 || string(raw) == "null" {
		return ai.Usage{}
	}

	return fields.extract(raw)
}

// intAtPath walks a dot-separated path through nested JSON objects and returns the integer leaf, or
// 0 if the path is absent or not a number. Values outside a sane token range are treated as 0 so a
// malformed provider payload cannot inject garbage (out-of-range float→int is implementation-defined).
func intAtPath(m map[string]any, path string) int {
	var cur any = m

	for key := range strings.SplitSeq(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return 0
		}

		cur, ok = obj[key]
		if !ok {
			return 0
		}
	}

	f, ok := cur.(float64)
	if !ok || f < 0 || f > math.MaxInt32 {
		return 0
	}

	return int(f)
}

type streamChunk struct {
	Choices []streamChoice  `json:"choices"`
	Usage   json.RawMessage `json:"usage"`
	Error   *wireError      `json:"error"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type streamDelta struct {
	Content   string                `json:"content"`
	ToolCalls []streamToolCallDelta `json:"tool_calls"`
}

// streamToolCallDelta is a partial tool call: fields arrive across chunks, keyed by Index, and the
// Function.Arguments string is concatenated fragment by fragment.
type streamToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func toWireMessages(messages []ai.Message) []wireMessage {
	out := make([]wireMessage, len(messages))

	for i := range messages {
		out[i] = wireMessage{
			Role:       messages[i].Role,
			Content:    messages[i].Content,
			ToolCallID: messages[i].ToolCallID,
			ToolCalls:  toWireRequestTools(messages[i].ToolCalls),
		}
	}

	return out
}

func toWireRequestTools(calls []ai.ToolCall) []wireRequestTool {
	if len(calls) == 0 {
		return nil
	}

	out := make([]wireRequestTool, len(calls))

	for i := range calls {
		out[i] = wireRequestTool{
			ID:   calls[i].ID,
			Type: functionType,
			Function: wireRequestFunc{
				Name:      calls[i].Name,
				Arguments: string(calls[i].Args),
			},
		}
	}

	return out
}

func toWireTools(tools []ai.ToolSpec) []wireTool {
	if len(tools) == 0 {
		return nil
	}

	out := make([]wireTool, len(tools))

	for i := range tools {
		out[i] = wireTool{
			Type: functionType,
			Function: wireFuncSpec{
				Name:        tools[i].Name,
				Description: tools[i].Description,
				Parameters:  tools[i].InputSchema,
			},
		}
	}

	return out
}

// toResponse maps a decoded chat response to ai.Response using the default usage mapping. A caller
// with custom UsageFields overrides resp.Usage afterward.
func toResponse(cr *chatResponse) *ai.Response {
	resp := &ai.Response{
		Model: cr.Model,
		Usage: mapUsage(UsageFields{}, cr.Usage),
	}

	if len(cr.Choices) == 0 {
		return resp
	}

	resp.Content = cr.Choices[0].Message.Content
	resp.ToolCalls = toToolCalls(cr.Choices[0].Message.ToolCalls)

	return resp
}

func toToolCalls(calls []wireResponseTool) []ai.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := make([]ai.ToolCall, len(calls))

	for i := range calls {
		// Keep Args valid JSON: empty (zero-arg tools) or malformed arguments normalize to {}.
		args := calls[i].Function.Arguments
		if args == "" || !json.Valid([]byte(args)) {
			args = "{}"
		}

		out[i] = ai.ToolCall{
			ID:   calls[i].ID,
			Name: calls[i].Function.Name,
			Args: json.RawMessage(args),
		}
	}

	return out
}
