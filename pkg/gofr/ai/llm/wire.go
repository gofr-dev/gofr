package llm

import (
	"encoding/json"

	"gofr.dev/pkg/gofr/ai"
)

const functionType = "function"

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Tools       []wireTool    `json:"tools,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
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
	Model   string       `json:"model"`
	Choices []wireChoice `json:"choices"`
	Usage   wireUsage    `json:"usage"`
	Error   *wireError   `json:"error"`
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

type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type streamChunk struct {
	Choices []streamChoice `json:"choices"`
	Usage   *wireUsage     `json:"usage"`
	Error   *wireError     `json:"error"`
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

func toResponse(cr *chatResponse) *ai.Response {
	resp := &ai.Response{
		Model: cr.Model,
		Usage: ai.Usage{
			PromptTokens:     cr.Usage.PromptTokens,
			CompletionTokens: cr.Usage.CompletionTokens,
		},
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
