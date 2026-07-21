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
	modelsPath          = "models"

	headerContentType   = "Content-Type"
	headerAuthorization = "Authorization"
	contentTypeJSON     = "application/json"

	opDial = "dial"
)

var (
	_ ai.Model          = (*Client)(nil)
	_ ai.StreamingModel = (*Client)(nil)
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

	apiKey  string
	baseURL string
	svc     service.HTTP
	logger  service.Logger
	metrics service.Metrics
	config  config.Config

	healthMu     sync.Mutex
	healthExpiry time.Time
	healthCache  datasource.Health
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

	body, err := c.buildRequest(messages, opts, false)
	if err != nil {
		return nil, err
	}

	resp, err := c.post(ctx, chatCompletionsPath, body)
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

// HealthCheck reports provider reachability, caching the result for a short TTL. The API key is
// never included in the returned details.
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
