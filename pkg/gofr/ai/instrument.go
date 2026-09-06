package ai

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	metricRequestCount     = "app_llm_request_count"
	metricTokensPerRequest = "app_llm_tokens_per_request" //nolint:gosec // G101: metric name, not a credential

	statusSuccess = "success"
	statusError   = "error"

	// traceIDMarker is the key GoFr's logger lifts to a top-level trace_id field, so an LLM log line
	// correlates to its request through the same key as GoFr's request and error logs.
	traceIDMarker = "__trace_id__"

	opGenerate = "generate"
	opChat     = "chat"
	opStream   = "stream"
	opEmbed    = "embed"

	tokenTypePrompt     = "prompt"
	tokenTypeCompletion = "completion"
	// cached and reasoning are subsets of prompt and completion respectively, recorded as their own
	// token_type series so a cache-hit rate (cached / prompt) can be derived without double-counting.
	tokenTypeCached    = "cached"
	tokenTypeReasoning = "reasoning"
)

// RegisterMetrics defines the LLM metrics. It is safe to call more than once; the framework manager
// logs and skips duplicate names.
//
// The tokens histogram carries a token_type label of prompt, completion, cached or reasoning, and
// its Prometheus _sum per token_type is the cumulative count for that type. Because cached is a
// subset of prompt and reasoning a subset of completion, a sum across all token types double-counts;
// filter to token_type=~"prompt|completion" for billable totals. Cost is
// (sum(prompt) - sum(cached))*in_rate + sum(cached)*cached_rate + sum(completion)*out_rate, and the
// cache-hit rate is sum(cached) / sum(prompt).
func RegisterMetrics(r MetricRegistrar) {
	// Buckets span single-token replies to very large context windows.
	tokenBuckets := []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000, 100000}

	r.NewCounter(metricRequestCount, "Number of LLM requests by provider, model, operation and status.")
	r.NewHistogram(metricTokensPerRequest,
		"Tokens per LLM request by token_type (prompt, completion, cached⊆prompt, reasoning⊆completion) and status.",
		tokenBuckets...)
}

// Instrument runs fn as a single LLM call, recording the span, metrics and log around it. It never
// records prompt or completion content — only counts and low-cardinality labels. fn receives the
// span-scoped context so the provider's own spans nest under this call.
//
// Instrument, StartCall and Recorder are the sanctioned extension points for provider modules that
// wrap or implement a Model and want their calls recorded with GoFr's LLM observability; the
// ctx.LLM() wrapper uses them internally.
func Instrument(ctx context.Context, info *CallInfo,
	fn func(ctx context.Context) (*Response, error)) (*Response, error) {
	ctx, span := tracerOf(info).Start(ctx, "llm."+info.Op)
	defer span.End()

	setBaseAttributes(span, info)

	resp, err := fn(ctx)
	record(ctx, info, span, resp, err)

	// On error the handler gets no response. Usage from a failed-but-billed call has already been
	// recorded to metrics above; a partial/empty response must not additionally leak into the
	// caller's HTTP body (which GoFr would otherwise render as 206 Partial Content beside the error).
	if err != nil {
		return nil, err
	}

	return resp, err
}

// Recorder captures a call whose usage is known only after it finishes, such as a stream drained
// over time. Callers must invoke Finish exactly once.
type Recorder struct {
	ctx  context.Context
	span trace.Span
	info *CallInfo
}

// StartCall opens the span for a deferred-usage call and returns a Recorder to close it.
func StartCall(ctx context.Context, info *CallInfo) *Recorder {
	ctx, span := tracerOf(info).Start(ctx, "llm."+info.Op)
	setBaseAttributes(span, info)

	return &Recorder{ctx: ctx, span: span, info: info}
}

// Context returns the span-scoped context so downstream work joins the trace.
func (r *Recorder) Context() context.Context { return r.ctx }

// Finish records usage and the outcome, then ends the span.
func (r *Recorder) Finish(u Usage, err error) {
	record(r.ctx, r.info, r.span, &Response{Usage: u}, err)
	r.span.End()
}

func record(ctx context.Context, info *CallInfo, span trace.Span, resp *Response, err error) {
	status := statusSuccess
	if err != nil {
		status = statusError

		span.RecordError(err)
		span.SetStatus(codes.Error, "")
	}

	if m := info.Deps.Metrics; m != nil {
		m.IncrementCounter(ctx, metricRequestCount,
			"provider", info.Provider, "model", info.Model, "operation", info.Op, "status", status)

		// Tokens are recorded whenever the provider reported them, tagged with the call's status, so
		// failed-but-billed spend (a 200-with-error-object, or a stream that fails mid-drain) stays
		// visible instead of being dropped. token_type="prompt",status="success" keeps its original
		// meaning and _count semantics; billable totals sum across statuses. recordTokens skips
		// zeros, so an error that reported no usage adds no series.
		if resp != nil {
			recordTokens(ctx, m, info, resp.Usage, status)
		}
	}

	if resp != nil {
		span.SetAttributes(
			attribute.Int("llm.tokens.prompt", resp.Usage.PromptTokens),
			attribute.Int("llm.tokens.completion", resp.Usage.CompletionTokens),
			attribute.Int("llm.tokens.total", resp.Usage.TotalTokens),
			attribute.Int("llm.tokens.cached", resp.Usage.CachedTokens),
			attribute.Int("llm.tokens.reasoning", resp.Usage.ReasoningTokens),
		)
	}

	writeLog(span, info, resp, err, status)
}

func recordTokens(ctx context.Context, m Metrics, info *CallInfo, u Usage, status string) {
	emit := func(tokenType string, value int) {
		if value > 0 {
			m.RecordHistogram(ctx, metricTokensPerRequest, float64(value),
				"provider", info.Provider, "model", info.Model, "token_type", tokenType, "status", status)
		}
	}

	emit(tokenTypePrompt, u.PromptTokens)
	emit(tokenTypeCompletion, u.CompletionTokens)
	emit(tokenTypeCached, u.CachedTokens)
	emit(tokenTypeReasoning, u.ReasoningTokens)
}

func writeLog(span trace.Span, info *CallInfo, resp *Response, err error, status string) {
	l := info.Deps.Logger
	if l == nil {
		return
	}

	entry := Log{Provider: info.Provider, Model: info.Model, Operation: info.Op, Status: status}
	if resp != nil {
		entry.PromptTokens = resp.Usage.PromptTokens
		entry.CompletionTokens = resp.Usage.CompletionTokens
		entry.TotalTokens = resp.Usage.TotalTokens
		entry.CachedTokens = resp.Usage.CachedTokens
		entry.ReasoningTokens = resp.Usage.ReasoningTokens
	}

	args := []any{entry}

	// Emit the trace ID the way every other GoFr log line does: as a __trace_id__ marker that the
	// framework logger lifts to a top-level trace_id field, so an LLM log correlates to its request
	// through the same key as GoFr's request and error logs. A noop tracer yields an invalid span
	// context, so the marker is omitted rather than emitting a zero trace ID.
	if sc := span.SpanContext(); sc.IsValid() {
		args = append(args, map[string]any{traceIDMarker: sc.TraceID().String()})
	}

	if err != nil {
		l.Error(args...)
		return
	}

	l.Debug(args...)
}

func setBaseAttributes(span trace.Span, info *CallInfo) {
	span.SetAttributes(
		attribute.String("llm.provider", info.Provider),
		attribute.String("llm.model", info.Model),
		attribute.String("llm.operation", info.Op),
	)
}

func tracerOf(info *CallInfo) trace.Tracer {
	if info.Deps.Tracer != nil {
		return info.Deps.Tracer
	}

	return noop.NewTracerProvider().Tracer("gofr-llm")
}

// Log is the structured record emitted for each LLM call. It never carries prompt or completion
// text, only metadata safe to log.
type Log struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Operation        string `json:"operation"`
	Status           string `json:"status"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens,omitempty"`
	CachedTokens     int    `json:"cachedTokens,omitempty"`
	ReasoningTokens  int    `json:"reasoningTokens,omitempty"`
}
