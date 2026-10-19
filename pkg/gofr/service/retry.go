package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	retryCountMetric           = "app_http_retry_count"
	retryWaitMetric            = "app_http_retry_wait_seconds"
	retryBudgetExhaustedMetric = "app_http_retry_budget_exhausted_total"
)

// RetryConfig retries failed calls to an HTTP service, up to MaxRetries times. With only MaxRetries set,
// it retries right away on any error or a status above 500, whatever the method.
//
// The other fields are for upstreams that can be overloaded: they back off, honor Retry-After, retry only
// requests that are safe to send twice, bound each attempt, and cap retries with a budget. Each one's
// zero value keeps the behavior above, so they are independent: Backoff alone still replays a POST and
// does not retry a 429. A safe setup also sets Methods and RetryOn: StandardRetryable.
//
// Setting any field besides MaxRetries also changes three things at once (MaxRetryAfter and
// IdempotencyKeyHeader do not count on their own, since they only qualify RespectRetryAfter and Methods): each retry is counted in
// app_http_retry_count with a reason label, its wait is recorded in app_http_retry_wait_seconds, and it
// is added to the caller's span and logged at debug level (instead of app_http_retry_count{service}
// being counted after each failed retry); an open circuit breaker is no longer retried; and a retry is
// skipped when the caller's deadline has no room for it.
type RetryConfig struct {
	// MaxRetries is the number of retries after the first attempt. 0 or less makes a single attempt.
	MaxRetries int
	// Backoff gives the wait before each retry. nil means no wait.
	Backoff Backoff
	// RetryOn decides whether an attempt's result is retried. nil means any error, or a status above 500.
	// StandardRetryable is the rule for an upstream that can be overloaded. Once any field besides
	// MaxRetries is set, an open circuit breaker and the caller's own context ending are never retried,
	// whatever it returns.
	RetryOn func(*http.Response, error) bool
	// RespectRetryAfter waits for the server's Retry-After on a 429 or 503, and for
	// RateLimitError.RetryAfter, instead of the backoff.
	RespectRetryAfter bool
	// MaxRetryAfter caps the honored Retry-After. It is used only when RespectRetryAfter is set. Default 30s.
	MaxRetryAfter time.Duration
	// Methods are the methods that are always retried. nil means every method. When set, a request with
	// any other method is sent once, unless it carries the idempotency key header; an empty, non-nil
	// list retries only requests that carry it.
	Methods []string
	// IdempotencyKeyHeader names the header that makes any other request, such as a POST, safe to retry.
	// It is used only when Methods is set. Default "Idempotency-Key".
	IdempotencyKeyHeader string
	// PerAttemptTimeout bounds how long one attempt waits for the response headers, so a hung attempt is
	// given up for a retry. 0 means no bound. It never cuts off an attempt no retry would follow: the
	// last one, any attempt of a request that may not be retried, and an attempt whose timer finds, when
	// it fires, that RetryOn does not retry a timeout, the budget is spent, or the caller's deadline has
	// no room for another attempt. Such an attempt runs until the upstream answers or the caller's
	// context ends. Reading the body of the returned response is not bounded by it; reading a response
	// before a retry is. With no PerAttemptTimeout, only the caller's context bounds that read, so an
	// upstream that stalls a response body holds the call until the context ends.
	PerAttemptTimeout time.Duration
	// Budget caps the retries made to the service. nil means unlimited.
	Budget *RetryBudget
}

// AddOption wraps h so its calls are retried.
func (r *RetryConfig) AddOption(h HTTP) HTTP {
	s := r.resolve()
	if s.legacy {
		return newRetryProvider(h, s, noWait)
	}

	return newRetryProvider(h, s, waitWithContext)
}

// legacyRetryable is RetryConfig's default rule: any error, or a status above 500.
func legacyRetryable(resp *http.Response, err error) bool {
	return err != nil || resp.StatusCode > http.StatusInternalServerError
}

// noWait is the wait of a RetryConfig with only MaxRetries set. It never fails, not even for a done
// context: the next attempt is then made and is the last, which is what the caller has always been handed.
func noWait(context.Context, time.Duration) error { return nil }

func newRetryProvider(h HTTP, s *retrySettings, wait func(context.Context, time.Duration) error) *retryProvider {
	rp := &retryProvider{
		HTTP:      h,
		settings:  s,
		wait:      wait,
		afterFunc: func(d time.Duration, f func()) stopper { return time.AfterFunc(d, f) },
	}

	if httpSvc := extractHTTPService(h); httpSvc != nil {
		rp.metrics = httpSvc.Metrics
		rp.serviceName = httpSvc.name

		if l, ok := httpSvc.Logger.(debugLogger); ok {
			rp.logger = l
		}
	}

	return rp
}

// retryProvider retries calls to the wrapped service under settings resolved from a RetryConfig.
type retryProvider struct {
	settings    *retrySettings
	wait        func(context.Context, time.Duration) error
	afterFunc   func(time.Duration, func()) stopper
	metrics     Metrics
	logger      debugLogger
	serviceName string
	HTTP
}

// stopper is the part of *time.Timer the per-attempt timeout uses.
type stopper interface {
	Stop() bool
}

func (rp *retryProvider) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response,
	error) {
	return rp.doWithRetry(ctx, http.MethodGet, nil, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.Get(ctx, path, queryParams)
	})
}

func (rp *retryProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodGet, headers, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	})
}

func (rp *retryProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodPost, nil, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.Post(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodPost, headers, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Put(ctx context.Context, api string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodPut, nil, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.Put(ctx, api, queryParams, body)
	})
}

func (rp *retryProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodPut, headers, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodPatch, nil, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.Patch(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodPatch, headers, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodDelete, nil, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.Delete(ctx, path, body)
	})
}

func (rp *retryProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, http.MethodDelete, headers, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.DeleteWithHeaders(ctx, path, body, headers)
	})
}

func (rp *retryProvider) Query(ctx context.Context, path string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, methodQuery, nil, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.Query(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) QueryWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, methodQuery, headers, func(ctx context.Context) (*http.Response, error) {
		return rp.HTTP.QueryWithHeaders(ctx, path, queryParams, body, headers)
	})
}

// doWithRetry makes the request, then retries it as the settings allow. Whenever it stops early -- the
// request may not be retried, the budget is spent, or the wait would outlast the caller's context -- it
// returns the last attempt's result as it is, with the body unread.
//
// Retries stop as soon as the caller's context is done. Every further attempt would fail the same way
// without reaching the network, and would only be logged, recorded as another failed outbound call, and
// counted by an inner circuit breaker.
//
// The context is checked on both sides of the attempt, because the two cases are not the same one.
// Checking only before it misses the common case: a deadline that passes while an attempt is already on
// the wire. That attempt returns the context's error, the loop then reads a context that was live when
// it was sampled, and a whole extra request goes out.
func (rp *retryProvider) doWithRetry(ctx context.Context, method string, headers map[string]string,
	reqFunc func(context.Context) (*http.Response, error)) (*http.Response, error) {
	s := rp.settings
	canRetry := s.canRetry(method, headers)

	if s.budget != nil {
		s.budget.deposit()
	}

	for attempt := 1; ; attempt++ {
		// Before: if the context is already done, this attempt is the last, but it is still made.
		// Returning the previous attempt's result instead would change the error the caller sees.
		ctxDone := ctx.Err() != nil

		last := !canRetry || attempt >= s.maxAttempts

		a := rp.attempt(ctx, attempt, last, reqFunc)

		rp.countLegacyRetry(attempt, a)

		if last || ctxDone || contextEndedTheAttempt(ctx, a.err) || !rp.replace(ctx, attempt, a) {
			return a.resp, a.err
		}
	}
}

// replace reports whether a retry follows attempt number attempt. If so, it has waited for it, spent the
// budget, recorded it, and released the attempt's response. If not, the response is left as it was,
// readable, to be returned.
func (rp *retryProvider) replace(ctx context.Context, attempt int, a attemptResult) bool {
	reason, wait, ok := rp.planRetry(ctx, attempt, a)
	if !ok {
		return false
	}

	// The budget is spent before the wait, so concurrent calls cannot all wait for the one retry it
	// has left; a wait cut short gives it back.
	refund, ok := rp.withdrawBudget()
	if !ok {
		return false
	}

	bound := rp.bodyReadBound(a.resp)

	// The connection goes back to the pool during the wait; the response stays readable in case no
	// retry follows.
	if wait > 0 {
		bound(func() { detachBody(a.resp) })
	}

	if rp.wait(ctx, wait) != nil {
		refund()

		return false
	}

	rp.recordRetry(ctx, attempt, reason, wait)

	// The attempt is being replaced, so nobody reads this response: drain it so its connection goes
	// back to the pool instead of being abandoned.
	bound(func() { drainAndCloseResponse(a.resp) })

	return true
}

// attemptResult is the outcome of one attempt. timedOut reports that the per-attempt timeout cut it off
// for a retry; wait is then the wait chosen for that retry.
type attemptResult struct {
	resp     *http.Response
	err      error
	timedOut bool
	wait     time.Duration
}

// attempt makes attempt number attempt, bounded by the per-attempt timeout unless it is the last.
func (rp *retryProvider) attempt(ctx context.Context, attempt int, last bool,
	reqFunc func(context.Context) (*http.Response, error)) attemptResult {
	if last || rp.settings.perAttemptTimeout <= 0 {
		resp, err := reqFunc(ctx) //nolint:bodyclose // Handed on in the result, to the caller or a drain.

		return attemptResult{resp: resp, err: err}
	}

	return attemptWithTimeout(ctx, rp.settings.perAttemptTimeout, rp.afterFunc,
		func() (time.Duration, bool) { return rp.retryAfterTimeout(ctx, attempt) }, reqFunc)
}

// retryAfterTimeout is asked when the per-attempt timer of attempt number attempt fires. It reports
// whether a retry could follow right now, and the wait before it. If not, the attempt is left running:
// cutting it off would only turn the upstream's answer into a timeout.
//
// The budget is only checked here, not spent. If another call spends the last of it before this
// retry does, this call returns the timeout.
//
// It runs on the timer's goroutine, where a panic in RetryOn or Backoff could not be recovered and would
// end the process. A panic counts as "no retry" instead: the attempt runs on, and whatever RetryOn does
// with its result then happens on the caller's goroutine.
func (rp *retryProvider) retryAfterTimeout(ctx context.Context, attempt int) (wait time.Duration, retry bool) {
	defer func() {
		if recover() != nil {
			wait, retry = 0, false
		}
	}()

	s := rp.settings

	if !s.retryOn(nil, errAttemptTimeout) {
		return 0, false
	}

	wait = s.waitBefore(attempt, nil, errAttemptTimeout)

	if !rp.roomForRetry(ctx, wait) || !rp.budgetAllows() {
		return 0, false
	}

	return wait, true
}

// planRetry decides whether a retry follows attempt number attempt, and if so, the reason to record and
// the wait before it. A retry is not made if the result is not retryable or the wait would outlast the
// caller's deadline. The budget is left to replace.
func (rp *retryProvider) planRetry(ctx context.Context, attempt int, a attemptResult) (reason string,
	wait time.Duration, ok bool) {
	s := rp.settings

	switch {
	case a.timedOut:
		// The timer already decided a retry follows, and chose its wait.
		reason, wait = retryReasonAttemptTimeout, a.wait
	case !s.retryable(a.resp, a.err):
		return "", 0, false
	default:
		reason, wait = reasonOf(a.resp, a.err), s.waitBefore(attempt, a.resp, a.err)

		if !rp.roomForRetry(ctx, wait) {
			return "", 0, false
		}
	}

	return reason, wait, true
}

// roomForRetry reports whether the caller's deadline leaves room for wait and, with a per-attempt
// timeout, one full attempt after it. A retry sent with a few milliseconds left would only swap the
// response in hand for the caller's deadline error.
//
// A RetryConfig with only MaxRetries set makes no such check: it retries until the context is done, then
// makes one last attempt.
func (rp *retryProvider) roomForRetry(ctx context.Context, wait time.Duration) bool {
	deadline, ok := ctx.Deadline()
	if rp.settings.legacy || !ok {
		return true
	}

	// Written as a subtraction from what is left, never wait + perAttemptTimeout: that sum can overflow
	// for a very long wait and wrap negative, which would pass the check.
	remaining, attemptTimeout := time.Until(deadline), rp.settings.perAttemptTimeout

	return remaining >= attemptTimeout && wait <= remaining-attemptTimeout
}

// bodyReadBound returns a function that runs a read of resp's body, bounded by the per-attempt timeout:
// if the read takes longer, the attempt's context is canceled, which aborts it. Without a per-attempt
// timeout, or for an attempt that has none, the read is not bounded.
func (rp *retryProvider) bodyReadBound(resp *http.Response) func(read func()) {
	c, ok := bodyOf(resp).(*cancelOnClose)
	if !ok || rp.settings.perAttemptTimeout <= 0 {
		return func(read func()) { read() }
	}

	return func(read func()) {
		// The timer cancels only a read still running: one that fires just as the read ends must not
		// cancel the context of a body that may yet be returned.
		var (
			mu   sync.Mutex
			done bool
		)

		t := rp.afterFunc(rp.settings.perAttemptTimeout, func() {
			mu.Lock()
			defer mu.Unlock()

			if !done {
				c.cancel(errAttemptTimeout)
			}
		})

		read()

		mu.Lock()
		done = true
		mu.Unlock()

		t.Stop()
	}
}

func bodyOf(resp *http.Response) io.ReadCloser {
	if resp == nil {
		return nil
	}

	return resp.Body
}

// budgetAllows reports whether the budget would allow a retry now, without spending it.
func (rp *retryProvider) budgetAllows() bool {
	b := rp.settings.budget

	return b == nil || b.allows()
}

// withdrawBudget spends one retry of the budget, and returns the function that gives it back if the
// retry is not made. A refusal is counted in app_http_retry_budget_exhausted_total.
func (rp *retryProvider) withdrawBudget() (refund func(), ok bool) {
	b := rp.settings.budget
	if b == nil {
		return func() {}, true
	}

	second, ok := b.tryWithdraw()
	if !ok {
		if rp.metrics != nil {
			rp.metrics.IncrementCounter(context.Background(), retryBudgetExhaustedMetric, "service", rp.serviceName)
		}

		return nil, false
	}

	return func() { b.refund(second) }, true
}

// countLegacyRetry counts attempt number attempt, for a RetryConfig with only MaxRetries set, if it was a
// retry that failed. The metric has no reason label, and a retry that succeeds is not counted.
func (rp *retryProvider) countLegacyRetry(attempt int, a attemptResult) {
	if rp.settings.legacy && attempt > 1 && rp.metrics != nil && legacyRetryable(a.resp, a.err) {
		rp.metrics.IncrementCounter(context.Background(), retryCountMetric, "service", rp.serviceName)
	}
}

// recordRetry reports that retry number retry is about to be made, after waiting wait. A RetryConfig with
// only MaxRetries set reports retries its own way, in countLegacyRetry.
func (rp *retryProvider) recordRetry(ctx context.Context, retry int, reason string, wait time.Duration) {
	if rp.settings.legacy {
		return
	}

	if rp.metrics != nil {
		rp.metrics.IncrementCounter(context.Background(), retryCountMetric, "service", rp.serviceName, "reason", reason)
		rp.metrics.RecordHistogram(context.Background(), retryWaitMetric, wait.Seconds(), "service", rp.serviceName)
	}

	trace.SpanFromContext(ctx).AddEvent("http.retry", trace.WithAttributes(
		attribute.Int("http.request.resend_count", retry),
		attribute.String("retry.reason", reason),
		attribute.Float64("retry.wait_seconds", wait.Seconds()),
	))

	if rp.logger != nil {
		rp.logger.Debugf("retrying call to service %q: retry %d, reason %s, after %v", rp.serviceName, retry, reason, wait)
	}
}

// contextEndedTheAttempt reports whether ctx is done AND is what ended the attempt that returned err.
//
// The second half is the load-bearing one. A bare "ctx.Err() != nil" here would swallow a connection
// error that merely coincided with a cancellation -- that is still a retryable connection error, and
// treating it as the context's doing changes the error the caller is handed.
func contextEndedTheAttempt(ctx context.Context, err error) bool {
	if ctx.Err() == nil {
		return false
	}

	return errors.Is(err, ctx.Err()) || errors.Is(err, context.Cause(ctx))
}
