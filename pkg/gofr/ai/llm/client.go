// Package llm provides an OpenAI-compatible LLM client that satisfies the gofr.dev/pkg/gofr/ai
// model contract. It rides GoFr's instrumented HTTP service so tracing, metrics, logging and
// connection pooling are inherited without extra wiring. Register a client with app.AddLLM:
//
//	app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b"})
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/service"
)

const (
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 20
	defaultIdleConnTimeout     = 90 * time.Second

	maxRetries         = 2
	baseBackoff        = 200 * time.Millisecond
	maxRetryAfter      = 5 * time.Second
	healthTTL          = 5 * time.Second
	healthProbeTimeout = 3 * time.Second
	maxErrBodyLen      = 512

	chatCompletionsPath = "chat/completions"
	embeddingsPath      = "embeddings"
	modelsPath          = "models"

	headerContentType   = "Content-Type"
	headerAuthorization = "Authorization"
	contentTypeJSON     = "application/json"

	opDial = "dial"
)

var (
	_ ai.Model          = (*Client)(nil)
	_ ai.StreamingModel = (*Client)(nil)
	_ ai.Embedder       = (*Client)(nil)
	_ ai.Descriptor     = (*Client)(nil)
)

// Client is a struct-direct OpenAI-compatible model. Exported fields are set by the caller; the
// framework fills the rest through the UseLogger/UseMetrics/UseConfig/Connect lifecycle.
type Client struct {
	// Provider selects the target service and its default base URL and API-key environment variable.
	Provider Provider
	// Model is the provider model identifier sent on every request.
	Model string
	// BaseURL overrides the provider default endpoint; leave empty to use the provider default.
	BaseURL string
	// UsageFields optionally remaps the JSON paths token counts are read from, for OpenAI-compatible
	// providers whose usage object deviates from the standard shape. The zero value uses the built-in
	// mapping (OpenAI/Groq/DeepSeek), so the popular providers need no configuration.
	UsageFields UsageFields
	// MaxConcurrentRequests caps the number of in-flight requests to the provider. When the cap is
	// reached, further Chat/Embed/Stream calls block until a slot frees (or their context is done) —
	// backpressure so a burst of concurrent agent calls cannot pile onto a provider that serializes
	// internally and turn every request's tail latency pathological. HealthCheck is never limited.
	// Zero (the default) means unlimited.
	MaxConcurrentRequests int

	apiKey  string
	baseURL string
	svc     service.HTTP
	logger  service.Logger
	metrics service.Metrics
	config  config.Config
	sem     chan struct{} // in-flight limiter; nil when MaxConcurrentRequests <= 0

	healthMu     sync.Mutex
	healthExpiry time.Time
	healthCache  datasource.Health
}

// acquire blocks until an in-flight slot is free (or ctx is done) when a concurrency cap is set, and
// returns a release that frees the slot. Both are no-ops when no cap is configured. The returned
// release is idempotent, so a streamer may call it on both exhaustion and Close.
func (c *Client) acquire(ctx context.Context) (release func(), err error) {
	if c.sem == nil {
		return func() {}, nil
	}

	select {
	case c.sem <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-c.sem }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// UseLogger wires the framework logger into the underlying HTTP service.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(service.Logger); ok {
		c.logger = l
	}
}

// UseMetrics wires the framework metrics manager into the underlying HTTP service.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(service.Metrics); ok {
		c.metrics = m
	}
}

// UseConfig reads the provider API key and resolves the base URL from the application config.
func (c *Client) UseConfig(cfg any) {
	if conf, ok := cfg.(config.Config); ok {
		c.config = conf
	}

	c.resolve()
}

// Connect builds the instrumented HTTP service used for all provider calls. If the provider is
// unknown and no BaseURL was set, no service is built so calls fail fast with a clear error.
func (c *Client) Connect() {
	c.resolve()

	if c.baseURL == "" {
		if c.logger != nil {
			c.logger.Log(fmt.Sprintf("%v: %q", errUnknownProvider, c.Provider))
		}

		return
	}

	if c.MaxConcurrentRequests > 0 {
		c.sem = make(chan struct{}, c.MaxConcurrentRequests)
	}

	c.svc = service.NewHTTPService(c.baseURL, c.logger, c.metrics,
		&service.ConnectionPoolConfig{
			MaxIdleConns:        defaultMaxIdleConns,
			MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
			IdleConnTimeout:     defaultIdleConnTimeout,
		},
		service.WithAttributes(map[string]string{"name": "llm-" + string(c.Provider)}),
	)
}

func (c *Client) resolve() {
	def, ok := providerDefaults(c.Provider)

	// Precedence for the endpoint: the BaseURL field, then LLM_BASE_URL from config, then the
	// provider default.
	c.baseURL = c.BaseURL
	if c.baseURL == "" && c.config != nil {
		c.baseURL = c.config.Get(envBaseURL)
	}

	if c.baseURL == "" && ok {
		c.baseURL = def.baseURL
	}

	if c.apiKey != "" {
		return
	}

	// Prefer the generic LLM_API_KEY (consistent with LLM_PROVIDER/LLM_MODEL) and fall back to the
	// provider-specific key (OPENAI_API_KEY, GROQ_API_KEY, ...) so existing conventions still work.
	c.apiKey = c.lookupKey(envAPIKey)
	if c.apiKey == "" && ok {
		c.apiKey = c.lookupKey(def.envVar)
	}
}

// lookupKey reads a key through the injected config only. GoFr's config is env-backed, so this
// still resolves environment variables without the client reaching for os.Getenv itself.
func (c *Client) lookupKey(env string) string {
	if c.config == nil {
		return ""
	}

	return c.config.Get(env)
}

// Name returns the provider label used as the health key and tracer name.
func (c *Client) Name() string { return string(c.Provider) }

// ProviderName returns the provider label for metrics and traces.
func (c *Client) ProviderName() string { return string(c.Provider) }

// ModelName returns the configured model identifier.
func (c *Client) ModelName() string { return c.Model }

// Chat sends a completion request and maps the provider response to an ai.Response.
func (c *Client) Chat(ctx context.Context, messages []ai.Message, opts ...ai.Option) (*ai.Response, error) {
	if c.svc == nil {
		return nil, errNotConnected
	}

	release, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	body, err := c.buildRequest(messages, opts, false)
	if err != nil {
		return nil, err
	}

	data, err := c.postJSON(ctx, chatCompletionsPath, body)
	if err != nil {
		return nil, err
	}

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("%w: %w", errDecodeResponse, err)
	}

	out := toResponse(&cr)

	if c.UsageFields.isSet() {
		out.Usage = mapUsage(&c.UsageFields, cr.Usage)
	}

	if cr.Error != nil {
		// Some OpenAI-compatible gateways report failures with HTTP 200 and an error object. Return
		// the usage they reported (the request was likely still billed) alongside the error so it is
		// recorded against the error status instead of being silently dropped.
		return &ai.Response{Usage: out.Usage}, fmt.Errorf("%w: %s", errProvider, cr.Error.Message)
	}

	return out, nil
}

// Embed satisfies ai.Embedder: it posts the input texts to the OpenAI-compatible /embeddings
// endpoint and returns one vector per input, in the same order. It rides the same instrumented
// HTTP service, retry and error handling as Chat.
//
// Options are accepted for signature parity with Chat and Stream but none currently apply, so they
// are deliberately ignored rather than silently half-honored: the options GoFr defines today
// (WithTemperature, WithMaxTokens, WithTools) are all completion parameters with no meaning for an
// embeddings request. The request body is therefore fixed at {model, input}.
//
// The ceiling that implies: the provider-side embedding parameters — notably `dimensions`, which
// drives Matryoshka truncation on text-embedding-3-*, and `encoding_format` — cannot be set through
// this client. Supporting them needs embedding-specific options plus the matching fields on
// embeddingsRequest; tracked in gofr-dev/gofr#3803.
func (c *Client) Embed(ctx context.Context, input []string, _ ...ai.Option) (*ai.EmbeddingResponse, error) {
	if c.svc == nil {
		return nil, errNotConnected
	}

	release, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	body, err := json.Marshal(embeddingsRequest{Model: c.Model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errEncodeRequest, err)
	}

	data, err := c.postJSON(ctx, embeddingsPath, body)
	if err != nil {
		return nil, err
	}

	var er embeddingsResponse
	if err = json.Unmarshal(data, &er); err != nil {
		return nil, fmt.Errorf("%w: %w", errDecodeResponse, err)
	}

	if er.Error != nil {
		return nil, fmt.Errorf("%w: %s", errProvider, er.Error.Message)
	}

	embeddings, err := placeEmbeddings(er.Data, len(input))
	if err != nil {
		return nil, err
	}

	return &ai.EmbeddingResponse{
		Model:      er.Model,
		Usage:      mapUsage(&c.UsageFields, er.Usage),
		Embeddings: embeddings,
	}, nil
}

// placeEmbeddings maps the response data array onto one vector per input, honoring each entry's
// "index" rather than its array position. The distinction only shows up on a provider that returns
// the array out of order — which the index field exists to allow — and there, positional mapping
// hands every input someone else's vector with nothing to signal it. That is a bad failure for
// embeddings: a wrong vector is still a valid vector, so semantic search and agent memory degrade
// silently instead of erroring.
//
// A missing index falls back to the entry's position, so providers that omit the field keep working.
// An index outside the input range, or one claimed twice, means the response cannot be mapped at all,
// and is reported instead of guessed at.
//
// Everything is validated against inputs — the number of texts sent — rather than against the length
// of the response, so a short response is caught for what it is. The embeddings endpoint returns one
// entry per input and reports failures for the whole request, so a count that disagrees is a broken
// response, and mapping it anyway would hand the caller a slice shorter than the inputs it was built
// from: an out-of-range panic for a caller indexing by input, or a silently skipped document for one
// ranging over the result.
func placeEmbeddings(data []embeddingDatum, inputs int) ([][]float32, error) {
	if len(data) != inputs {
		return nil, fmt.Errorf("%w: provider returned %d embeddings for %d inputs",
			errDecodeResponse, len(data), inputs)
	}

	out := make([][]float32, inputs)
	seen := make([]bool, inputs)

	for i := range data {
		pos := i
		if data[i].Index != nil {
			pos = *data[i].Index
		}

		if pos < 0 || pos >= inputs {
			return nil, fmt.Errorf("%w: embedding index %d outside the %d inputs sent",
				errDecodeResponse, pos, inputs)
		}

		if seen[pos] {
			return nil, fmt.Errorf("%w: embedding index %d returned more than once", errDecodeResponse, pos)
		}

		seen[pos] = true
		out[pos] = data[i].Embedding
	}

	return out, nil
}

// HealthCheck reports provider reachability, caching the result for a short TTL. The API key is
// never included in the returned details.
//
// The lock is deliberately held across the probe rather than released around it: that makes the
// probe single-flight, so a burst of concurrent health checks on a cold cache costs the provider one
// request instead of one per caller. The cost is that concurrent callers wait for the in-flight probe
// (bounded by healthProbeTimeout) instead of racing their own — the right trade for a call that is
// polled by /.well-known/health rather than served on a request path.
func (c *Client) HealthCheck(ctx context.Context) datasource.Health {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()

	if time.Now().Before(c.healthExpiry) {
		return cloneHealth(c.healthCache)
	}

	health := c.probe(ctx)

	// Do not cache a result derived from an already-failing caller context; a fine provider would
	// otherwise be reported down for the whole TTL.
	if ctx.Err() == nil {
		c.healthCache = health
		c.healthExpiry = time.Now().Add(healthTTL)
	}

	return cloneHealth(health)
}

func (c *Client) probe(ctx context.Context) datasource.Health {
	health := datasource.Health{
		Status: datasource.StatusDown,
		Details: map[string]any{
			"provider": string(c.Provider),
			"model":    c.Model,
			"base_url": c.baseURL,
		},
	}

	if c.svc == nil {
		return health
	}

	pctx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
	defer cancel()

	resp, err := c.svc.GetWithHeaders(pctx, modelsPath, nil, c.headers())
	if err != nil {
		return health
	}
	defer drain(resp)

	if isSuccess(resp.StatusCode) {
		health.Status = datasource.StatusUp
	}

	return health
}

func (c *Client) buildRequest(messages []ai.Message, opts []ai.Option, stream bool) ([]byte, error) {
	co := ai.ApplyOptions(opts...)

	req := chatRequest{
		Model:       c.Model,
		Messages:    toWireMessages(messages),
		Temperature: co.Temperature,
		MaxTokens:   co.MaxTokens,
		Tools:       toWireTools(co.Tools),
		Stream:      stream,
	}

	// Ask for the final usage chunk; many providers omit it from streams unless requested.
	if stream {
		req.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errEncodeRequest, err)
	}

	return body, nil
}

func (c *Client) headers() map[string]string {
	h := map[string]string{headerContentType: contentTypeJSON}
	if c.apiKey != "" {
		h[headerAuthorization] = "Bearer " + c.apiKey
	}

	return h
}

// post sends the request with a small, condition-aware retry: connection errors that never reached
// the server and HTTP 429 are retried; a timed-out request is not retried (it may have been
// delivered) and a canceled context aborts immediately.
func (c *Client) post(ctx context.Context, path string, body []byte) (*http.Response, error) {
	headers := c.headers()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err := c.svc.PostWithHeaders(ctx, path, nil, body, headers)

		if !shouldRetry(ctx, resp, err) || attempt == maxRetries {
			if err != nil {
				return nil, wrapPostErr(ctx, err)
			}

			return resp, nil
		}

		drain(resp)

		if !backoffSleep(ctx, retryDelay(resp, attempt)) {
			return nil, ctx.Err()
		}
	}

	return nil, errRequestFailed
}

// postJSON sends a JSON body to path and returns the raw success-response bytes, mapping transport,
// body-read and non-2xx status failures to errors. Decoding the success body is left to the caller,
// so Chat and Embed share it; Stream reads its body incrementally and does not.
func (c *Client) postJSON(ctx context.Context, path string, body []byte) ([]byte, error) {
	resp, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	defer drain(resp)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRequestFailed, err)
	}

	if !isSuccess(resp.StatusCode) {
		return nil, c.statusError(resp.StatusCode, data)
	}

	return data, nil
}

func shouldRetry(ctx context.Context, resp *http.Response, err error) bool {
	if ctx.Err() != nil {
		return false
	}

	if err != nil {
		return isRetriableConnErr(err)
	}

	return resp.StatusCode == http.StatusTooManyRequests
}

func wrapPostErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	return fmt.Errorf("%w: %w", errRequestFailed, err)
}

func isRetriableConnErr(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == opDial
	}

	return false
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		// Only the integer-seconds form is honored; the HTTP-date form falls back to backoff. A
		// server-supplied delay is capped so a large Retry-After cannot stall the caller for minutes.
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := strconv.Atoi(v); err == nil {
				// Clamp before converting to a Duration: secs*1e9 overflows int64 (wrapping negative)
				// for a large Retry-After, and a negative header must not yield a negative delay. The
				// delay is capped at maxRetryAfter regardless, so bounding secs to that loses nothing.
				secs = max(0, min(secs, int(maxRetryAfter/time.Second)))
				return time.Duration(secs) * time.Second
			}
		}
	}

	return baseBackoff + time.Duration(attempt)*baseBackoff
}

func backoffSleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func isSuccess(code int) bool {
	return code >= http.StatusOK && code < http.StatusMultipleChoices
}

// statusError wraps a non-2xx response with the status code and a truncated response body. The API
// key is redacted in case a misbehaving proxy echoes request headers in its error page.
func (c *Client) statusError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if len(msg) > maxErrBodyLen {
		msg = strings.ToValidUTF8(msg[:maxErrBodyLen], "")
	}

	if c.apiKey != "" {
		msg = strings.ReplaceAll(msg, c.apiKey, "[REDACTED]")
	}

	return fmt.Errorf("%w: status %d: %s", errUnexpectedStatus, status, msg)
}

// cloneHealth copies the health so a cached value's Details map is never shared with callers.
func cloneHealth(h datasource.Health) datasource.Health {
	details := make(map[string]any, len(h.Details))
	for k, v := range h.Details {
		details[k] = v
	}

	return datasource.Health{Status: h.Status, Details: details}
}

func drain(resp *http.Response) {
	if resp == nil {
		return
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
