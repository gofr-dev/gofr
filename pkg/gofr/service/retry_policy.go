package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxRetryAfter        = 30 * time.Second
	defaultIdempotencyKeyHeader = "Idempotency-Key"
	defaultBackoffMultiplier    = 2

	retryReasonStatus5xx      = "status_5xx"
	retryReasonStatus429      = "status_429"
	retryReasonStatus408      = "status_408"
	retryReasonStatusOther    = "status_other"
	retryReasonNetwork        = "network"
	retryReasonAttemptTimeout = "attempt_timeout"
)

// errAttemptTimeout is returned when an attempt abandoned by RetryConfig.PerAttemptTimeout is not retried.
// It wraps context.DeadlineExceeded, so it reads as the timeout it is.
var errAttemptTimeout = fmt.Errorf("attempt exceeded the retry per-attempt timeout: %w", context.DeadlineExceeded)

// debugLogger is the part of logging.Logger the retry layer uses. The service Logger only has Log.
type debugLogger interface {
	Debugf(format string, args ...any)
}

// retrySettings is a RetryConfig with its defaults filled in.
type retrySettings struct {
	// legacy marks a RetryConfig that sets only MaxRetries. It keeps three ways such a RetryConfig has
	// always had: app_http_retry_count{service} is counted after each failed attempt past the first, with
	// no reason label, wait histogram, span event or log; ErrCircuitOpen is retried; and a retry is made
	// whatever the caller's deadline leaves.
	legacy               bool
	maxAttempts          int
	backoff              Backoff
	retryOn              func(*http.Response, error) bool
	respectRetryAfter    bool
	maxRetryAfter        time.Duration
	methods              map[string]struct{} // nil: every method
	idempotencyKeyHeader string
	perAttemptTimeout    time.Duration
	budget               *retryBudget
}

// onlyMaxRetries reports whether every field besides MaxRetries is left at its zero value.
func (r *RetryConfig) onlyMaxRetries() bool {
	return r.Backoff == nil && r.RetryOn == nil && !r.RespectRetryAfter && r.MaxRetryAfter == 0 &&
		r.Methods == nil && r.IdempotencyKeyHeader == "" && r.PerAttemptTimeout == 0 && r.Budget == nil
}

func (r *RetryConfig) resolve() *retrySettings {
	s := &retrySettings{
		legacy:               r.onlyMaxRetries(),
		maxAttempts:          min(max(r.MaxRetries, 0), math.MaxInt-1) + 1,
		backoff:              r.Backoff,
		retryOn:              r.RetryOn,
		respectRetryAfter:    r.RespectRetryAfter,
		maxRetryAfter:        r.MaxRetryAfter,
		idempotencyKeyHeader: http.CanonicalHeaderKey(r.IdempotencyKeyHeader),
		perAttemptTimeout:    r.PerAttemptTimeout,
	}

	if s.retryOn == nil {
		s.retryOn = legacyRetryable
	}

	if s.maxRetryAfter <= 0 {
		s.maxRetryAfter = defaultMaxRetryAfter
	}

	if s.idempotencyKeyHeader == "" {
		s.idempotencyKeyHeader = defaultIdempotencyKeyHeader
	}

	if r.Methods != nil {
		s.methods = make(map[string]struct{}, len(r.Methods))
		for _, m := range r.Methods {
			s.methods[strings.ToUpper(m)] = struct{}{}
		}
	}

	if r.Budget != nil {
		s.budget = newRetryBudget(*r.Budget, time.Now)
	}

	return s
}

// canRetry reports whether a request may be sent more than once: its method is always retried, or it
// carries a non-empty idempotency key.
func (s *retrySettings) canRetry(method string, headers map[string]string) bool {
	if s.maxAttempts <= 1 {
		return false
	}

	if s.methods == nil {
		return true
	}

	if _, ok := s.methods[method]; ok {
		return true
	}

	for k, v := range headers {
		if v != "" && http.CanonicalHeaderKey(k) == s.idempotencyKeyHeader {
			return true
		}
	}

	return false
}

// retryable reports whether the result of an attempt is retried. Unless the RetryConfig sets only
// MaxRetries, an open circuit breaker is never retried, whatever RetryOn returns.
func (s *retrySettings) retryable(resp *http.Response, err error) bool {
	if !s.legacy && errors.Is(err, ErrCircuitOpen) {
		return false
	}

	return s.retryOn(resp, err)
}

func reasonOf(resp *http.Response, err error) string {
	var rle *RateLimitError

	switch {
	case errors.As(err, &rle):
		return retryReasonStatus429
	case err != nil || resp == nil:
		return retryReasonNetwork
	case resp.StatusCode == http.StatusTooManyRequests:
		return retryReasonStatus429
	case resp.StatusCode == http.StatusRequestTimeout:
		return retryReasonStatus408
	case resp.StatusCode >= http.StatusInternalServerError:
		return retryReasonStatus5xx
	default:
		return retryReasonStatusOther
	}
}

// waitBefore returns the wait before retry number retry (1 for the first retry).
func (s *retrySettings) waitBefore(retry int, resp *http.Response, err error) time.Duration {
	if s.respectRetryAfter {
		if d, ok := retryAfterOf(resp, err); ok {
			return min(d, s.maxRetryAfter)
		}
	}

	if s.backoff == nil {
		return 0
	}

	return max(s.backoff.Next(retry), 0)
}

func retryAfterOf(resp *http.Response, err error) (time.Duration, bool) {
	var rle *RateLimitError
	if errors.As(err, &rle) {
		return max(rle.RetryAfter, 0), true
	}

	if resp == nil || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable) {
		return 0, false
	}

	return parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
}

// parseRetryAfter reads a Retry-After value: a number of seconds or an HTTP date. A date in the past
// means no wait.
func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	if secs, err := strconv.ParseInt(value, 10, 64); err == nil {
		if secs < 0 {
			return 0, false
		}

		if secs > int64(math.MaxInt64/time.Second) {
			return time.Duration(math.MaxInt64), true
		}

		return time.Duration(secs) * time.Second, true
	}

	t, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}

	return max(t.Sub(now), 0), true
}

// Backoff gives the wait before a retry.
type Backoff interface {
	// Next returns the wait before retry number attempt; attempt is 1 for the first retry.
	Next(attempt int) time.Duration
}

// ConstantBackoff waits Delay before every retry.
type ConstantBackoff struct {
	Delay time.Duration
}

// Next returns Delay.
func (b ConstantBackoff) Next(int) time.Duration {
	return b.Delay
}

// ExponentialBackoff waits a random time between 0 and min(Max, Base × Multiplier^(attempt-1)) ("full
// jitter"). The randomness spreads the retries of many callers out, so they do not hit a recovering
// upstream at the same moment.
type ExponentialBackoff struct {
	// Base is the ceiling of the first wait.
	Base time.Duration
	// Max caps the ceiling. 0 means no cap.
	Max time.Duration
	// Multiplier is how much the ceiling grows per retry. Default 2.
	Multiplier float64
}

// Next returns a random wait between 0 and the ceiling for attempt.
func (b ExponentialBackoff) Next(attempt int) time.Duration {
	ceiling := b.ceiling(attempt)
	if ceiling <= 0 {
		return 0
	}

	if ceiling == math.MaxInt64 {
		return time.Duration(rand.Int64N(int64(ceiling))) //nolint:gosec // Jitter, not security.
	}

	return time.Duration(rand.Int64N(int64(ceiling) + 1)) //nolint:gosec // Jitter, not security.
}

func (b ExponentialBackoff) ceiling(attempt int) time.Duration {
	if b.Base <= 0 {
		return 0
	}

	multiplier := b.Multiplier
	if multiplier <= 0 {
		multiplier = defaultBackoffMultiplier
	}

	limit := float64(math.MaxInt64)
	if b.Max > 0 {
		limit = float64(b.Max)
	}

	c := float64(b.Base) * math.Pow(multiplier, float64(max(attempt, 1)-1))
	if c >= limit || math.IsNaN(c) {
		if b.Max > 0 {
			return b.Max
		}

		return time.Duration(math.MaxInt64)
	}

	return time.Duration(c)
}

// StandardRetryable is the RetryConfig.RetryOn for an upstream that can be overloaded. It retries
// network errors, a RateLimitError from RateLimiterConfig, and the statuses 408, 429, 502, 503 and 504.
// It never retries 500, 501 or 505, or an open circuit breaker. Any other error, including a timeout, counts as a network error; the
// caller's own context ending is never retried, whatever RetryOn returns.
//
// A custom rule can build on it:
//
//	RetryOn: func(resp *http.Response, err error) bool {
//		return service.StandardRetryable(resp, err) || (resp != nil && resp.StatusCode == http.StatusConflict)
//	}
func StandardRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return !errors.Is(err, ErrCircuitOpen)
	}

	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// waitWithContext waits d, or until ctx is done, whichever is first.
func waitWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// attemptWithTimeout makes one attempt under a per-attempt timeout. When timeout passes before the
// response headers arrive, decide is asked whether a retry can follow; only then is the attempt's
// context canceled, and decide's wait is handed on for that retry. Once the headers are in, the context
// lives until the body is closed, so reading the body is not cut off.
func attemptWithTimeout(ctx context.Context, timeout time.Duration, afterFunc func(time.Duration, func()) stopper,
	decide func() (time.Duration, bool), reqFunc func(context.Context) (*http.Response, error)) attemptResult {
	cancelCtx, cancel := context.WithCancelCause(ctx)
	attemptCtx := attemptContext{Context: cancelCtx}

	var wait time.Duration

	decided := make(chan struct{})

	timer := afterFunc(timeout, func() {
		defer close(decided)

		var retry bool
		if wait, retry = decide(); retry {
			cancel(errAttemptTimeout)
		}
	})

	// The timer and context are released on every way out, a panic in reqFunc included, unless the
	// response body takes them over.
	release := true

	defer func() {
		if release {
			timer.Stop()
			cancel(nil)
		}
	}()

	resp, err := reqFunc(attemptCtx)

	// Stop reports false once the timer has fired, perhaps just as the headers arrived, and its callback
	// may still be deciding. Wait for it, so the cause read below is the one it settled on.
	if !timer.Stop() {
		<-decided
	}

	if ctx.Err() != nil || !errors.Is(context.Cause(cancelCtx), errAttemptTimeout) {
		if err == nil && resp != nil && resp.Body != nil {
			release = false
			resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
		}

		return attemptResult{resp: resp, err: err}
	}

	drainAndCloseResponse(resp)

	return attemptResult{err: attemptTimeoutError(timeout, err), timedOut: true, wait: wait}
}

// attemptTimeoutError is the error of an attempt the per-attempt timeout cut off. It keeps the
// transport's error (a *url.Error naming the request) when there is one.
func attemptTimeoutError(timeout time.Duration, err error) error {
	switch {
	case err == nil:
		return fmt.Errorf("%w (%v)", errAttemptTimeout, timeout)
	case !errors.Is(err, errAttemptTimeout):
		return fmt.Errorf("%w (%v): %w", errAttemptTimeout, timeout, err)
	default:
		return err
	}
}

// attemptContext is the context of one attempt. When the per-attempt timeout ends it, Err reports
// context.DeadlineExceeded rather than context.Canceled: the attempt ran out of time, nobody canceled
// it. An inner circuit breaker relies on this to count the timeout as a failure of the upstream.
type attemptContext struct {
	context.Context
}

func (c attemptContext) Err() error {
	err := c.Context.Err()
	if err != nil && errors.Is(context.Cause(c.Context), errAttemptTimeout) {
		return context.DeadlineExceeded
	}

	return err
}

// detachBody reads a small response body into memory and closes the original, so the connection goes
// back to the pool while the retry waits, and the response can still be returned intact if no retry is
// made. A body larger than discardedBodyLimit is left as it is.
func detachBody(resp *http.Response) {
	if resp == nil || resp.Body == nil || resp.Body == http.NoBody {
		return
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, discardedBodyLimit+1))

	switch {
	case err == nil && len(b) <= discardedBodyLimit:
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(b))
	default:
		// Too large or unreadable: put back what was read in front of the rest.
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(b), resp.Body), resp.Body}
	}
}

// cancelOnClose releases an attempt's context when the response body is closed.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelCauseFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel(nil)

	return err
}
