// Package ai defines the provider-agnostic contracts GoFr uses to talk to large language
// models and to expose service handlers as agent tools. Concrete providers live in their own
// modules and satisfy these interfaces; the framework supplies the observability around them.
package ai

import (
	"context"
	"encoding/json"

	"gofr.dev/pkg/gofr/datasource"
)

// Model is the minimal contract a provider implements. It is intentionally frozen and small;
// capabilities are added through new optional interfaces (see StreamingModel), never by growing
// this one.
type Model interface {
	Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error)
	HealthCheck(ctx context.Context) datasource.Health
	// Name is the single source of the provider label, health key, and tracer name.
	Name() string
}

// StreamingModel is implemented by providers that can stream a response incrementally.
type StreamingModel interface {
	Stream(ctx context.Context, messages []Message, opts ...Option) (Streamer, error)
}

// Descriptor is an optional interface a provider implements to report distinct provider and model
// labels for metrics and traces. Without it, Name() is used as the provider label.
type Descriptor interface {
	Provider() string
	ModelName() string
}

// LLM is what ctx.LLM() returns. Like Model it is frozen: new capabilities are added through new
// optional interfaces retrieved by type assertion, never by adding methods here, so hand-written
// fakes and third-party wrappers keep compiling across minor versions. Generate is a convenience
// over a single-message Chat.
type LLM interface {
	Model
	Generate(ctx context.Context, prompt string, opts ...Option) (*Response, error)
	Stream(ctx context.Context, messages []Message, opts ...Option) (Streamer, error)
	Tools() Tools
}

// Tools is the set of the service's own handlers exposed as agent-callable tools. It is frozen on
// the same terms as LLM; grow it via new optional interfaces.
type Tools interface {
	List() []ToolSpec
	Only(names ...string) Tools
	Call(ctx context.Context, name string, args json.RawMessage) (Result, error)
}

// Streamer is a generic pull iterator. It is structurally identical to http/response.Streamer so
// a stream can flow from provider to client without importing across the transport boundary.
type Streamer interface {
	Next() (any, bool)
	Err() error
	Close() error
}

// Response is a single model completion.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
	Model     string
}

// Usage reports token consumption for a call.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// Message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is one turn in a conversation. Role is one of "system", "user", "assistant", "tool".
// ToolCalls is set on assistant turns that invoke tools; ToolCallID links a "tool" turn to the
// call it answers.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// ToolCall is a model's request to invoke a named tool with JSON arguments.
type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

// ToolSpec describes a tool the model may call. InputSchema is JSON Schema derived from the
// handler's bound request.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Access      Access
}

// Result wraps a tool handler's return value for feeding back into the next message.
type Result struct {
	value any
}

// NewResult wraps a handler return value.
func NewResult(v any) Result { return Result{value: v} }

// Value returns the wrapped handler return value.
func (r Result) Value() any { return r.value }

// JSON marshals the wrapped value for inclusion in a follow-up message.
func (r Result) JSON() (json.RawMessage, error) { return json.Marshal(r.value) }

// Access classifies a tool's side effects. The zero value is ReadOnly so a tool is never
// exposed as writable unless it is declared so.
type Access int

const (
	// ReadOnly tools are safe to call and safe to retry.
	ReadOnly Access = iota
	// Write tools mutate state and are opt-in.
	Write
)

func (a Access) String() string {
	if a == Write {
		return "write"
	}

	return "read"
}
