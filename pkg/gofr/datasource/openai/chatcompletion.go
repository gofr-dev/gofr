package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const CompletionsEndpoint = "/v1/chat/completions"

type CreateCompletionsRequest struct {
	Messages            []Message         `json:"messages,omitempty"`
	Model               string            `json:"model,omitempty"`
	Store               bool              `json:"store,omitempty"`
	ReasoningEffort     string            `json:"reasoning_effort,omitempty"`
	MetaData            interface{}       `json:"metadata,omitempty"` // object or null
	FrequencyPenalty    float64           `json:"frequency_penalty,omitempty"`
	LogitBias           map[string]string `json:"logit_bias,omitempty"`
	LogProbs            int               `json:"logprobs,omitempty"`
	TopLogProbs         int               `json:"top_logprobs,omitempty"`
	MaxTokens           int               `json:"max_tokens,omitempty"` // deprecated
	MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
	N                   int               `json:"n,omitempty"`
	Modalities          []string          `json:"modalities,omitempty"`
	Prediction          interface{}       `json:"prediction,omitempty"`
	PresencePenalty     float64           `json:"presence_penalty,omitempty"`

	Audio struct {
		Voice  string `json:"voice,omitempty"`
		Format string `json:"format,omitempty"`
	} `json:"audio,omitempty"`

	ResposneFormat interface{} `json:"response_format,omitempty"`
	Seed           int         `json:"seed,omitempty"`
	ServiceTier    string      `json:"service_tier,omitempty"`
	Stop           interface{} `json:"stop,omitempty"`
	Stream         bool        `json:"stream,omitempty"`

	StreamOptions struct {
		IncludeUsage bool `json:"include_usage,omitempty"`
	} `json:"stram_options,omitempty"`

	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`

	Tools []struct {
		Type     string `json:"type,omitempty"`
		Function struct {
			Name        string      `json:"name,omitempty"`
			Description string      `json:"description,omitempty"`
			Parameters  interface{} `json:"parameters,omitempty"`
			Strict      bool        `json:"strict,omitempty"`
		} `json:"function,omitempty"`
	} `json:"tools,omitempty"`

	ToolChoice        interface{} `json:"tool_choice,omitempty"`
	ParallelToolCalls bool        `json:"parallel_tool_calls,omitempty"`
	Suffix            string      `json:"suffix,omitempty"`
	User              string      `json:"user,omitempty"`
}

type Message struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
}

type CreateCompletionsResponse struct {
	ID                string `json:"id,omitempty"`
	Object            string `json:"object,omitempty"`
	Created           int    `json:"created,omitempty"`
	Model             string `json:"model,omitempty"`
	ServiceTier       string `json:"service_tier,omitempty"`
	SystemFingerprint string `json:"system_fingerprint,omitempty"`

	Choices []struct {
		Index int `json:"index,omitempty"`

		Message struct {
			Role      string      `json:"role,omitempty"`
			Content   string      `json:"content,omitempty"`
			Refusal   string      `json:"refusal,omitempty"`
			ToolCalls interface{} `json:"tool_calls,omitempty"`
		} `json:"message"`

		Logprobs     interface{} `json:"logprobs,omitempty"`
		FinishReason string      `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`

	Usage struct {
		PromptTokens           int         `json:"prompt_tokens,omitempty"`
		CompletionTokens       int         `json:"completion_tokens,omitempty"`
		TotalTokens            int         `json:"total_tokens,omitempty"`
		CompletionTokensDetails interface{} `json:"completion_tokens_details,omitempty"`
		PromptTokensDetails     interface{} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage,omitempty"`

	Error *Error `json:"error,omitempty"`
}

type Error struct {
	Message string      `json:"message,omitempty"`
	Type    string      `json:"type,omitempty"`
	Param   interface{} `json:"param,omitempty"`
	Code    interface{} `json:"code,omitempty"`
}

var (
	ErrMissingBoth     = errors.New("both messages and model fields not provided")
	ErrMissingMessages = errors.New("messages fields not provided")
	ErrMissingModel    = errors.New("model fields not provided")
)

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (c *Client) CreateCompletionsRaw(ctx context.Context, r *CreateCompletionsRequest) ([]byte, error) {
	return c.Post(ctx, CompletionsEndpoint, r)
}

func (c *Client) CreateCompletions(ctx context.Context, r *CreateCompletionsRequest) (response *CreateCompletionsResponse, err error) {
	tracerCtx, span := c.AddTrace(ctx, "CreateCompletions")
	startTime := time.Now()

	if r.Messages == nil && r.Model == "" {
		c.logger.Errorf("%v", ErrMissingBoth)
		return nil, ErrMissingBoth
	}

	if r.Messages == nil {
		c.logger.Errorf("%v", ErrMissingMessages)
		return nil, ErrMissingMessages
	}

	if r.Model == "" {
		c.logger.Errorf("%v", ErrMissingModel)
		return nil, ErrMissingModel
	}

	raw, err := c.CreateCompletionsRaw(tracerCtx, r)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, err
	}

	ql := &APILog{
		ID:                response.ID,
		Object:            response.Object,
		Created:           response.Created,
		Model:             response.Model,
		ServiceTier:       response.ServiceTier,
		SystemFingerprint: response.SystemFingerprint,
		Usage:             response.Usage,
		Error:             response.Error,
	}

	c.SendChatCompletionOperationStats(ql, startTime, "ChatCompletion", span)

	return response, err
}

func (c *Client) SendChatCompletionOperationStats(ql *APILog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "openai_api_request_duration", float64(duration))
	c.metrics.RecordRequestCount(context.Background(), "openai_api_total_request_count")
	c.metrics.RecordTokenUsage(context.Background(), "openai_api_token_usage", ql.Usage.PromptTokens, ql.Usage.CompletionTokens)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("openai.%v.duration", method), duration))
	}
}
