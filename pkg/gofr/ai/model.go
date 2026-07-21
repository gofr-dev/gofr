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
// labels for metrics and traces. Without it, Name() is used for both labels. The methods are named
// ProviderName/ModelName (not Provider/Model) so a provider can expose Provider and Model as
// exported struct fields, which Go forbids alongside methods of the same name.
type Descriptor interface {
	ProviderName() string
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

// Streamer is a generic pull iterator. It conforms to the http/response.Streamer transport
// primitive so a model stream can be returned directly as a response.Stream source, without either
// package importing the other.
type Streamer interface {
	Next() (any, bool)
	Err() error
	Close() error
}

// ToolCallStreamer is an optional Streamer capability. A stream that carried tool calls assembles
// them from the provider's deltas and returns them here once Next has returned false. Call ToolCalls
// from the same goroutine that drained Next, after Next has returned false; it returns the completed
// tool calls, or nil if there were none. Note the stream returned by ctx.LLM().Stream always exposes
// this method (returning nil when unsupported), so treat a nil result — not the type assertion — as
// "no tool calls".
type ToolCallStreamer interface {
	ToolCalls() []ToolCall
}

// Response is a single model completion.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
	Model     string
}

// Usage reports token consumption for a call. PromptTokens and CompletionTokens are the input and
// output counts; the remaining fields are populated when the provider reports them and are zero
// otherwise. CachedTokens and ReasoningTokens are subsets of PromptTokens and CompletionTokens
// respectively, not additions to them.
type Usage struct {
	// PromptTokens is the number of input (prompt) tokens.
	PromptTokens int
	// CompletionTokens is the number of output (completion) tokens.
	CompletionTokens int
	// TotalTokens is the provider-reported total; it may exceed PromptTokens+CompletionTokens when
	// the provider counts extras. Zero if not reported.
	TotalTokens int
	// CachedTokens is the subset of PromptTokens served from the provider's prompt cache (a
	// cache-read hit), billed at a lower rate. Zero if not reported or unsupported.
	CachedTokens int
	// ReasoningTokens is the subset of CompletionTokens spent on internal reasoning by reasoning
	// models. Zero if not reported.
	ReasoningTokens int
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
