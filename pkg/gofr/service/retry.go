package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	retryCountMetric           = "app_http_retry_count"
	retryWaitMetric            = "app_http_retry_wait_seconds"
	retryBudgetExhaustedMetric = "app_http_retry_budget_exhausted_total"
)

type RetryConfig struct {
	MaxRetries int
}

func (r *RetryConfig) AddOption(h HTTP) HTTP {
	rp := &retryProvider{
		maxRetries: r.MaxRetries,
		HTTP:       h,
	}

	if httpSvc := extractHTTPService(h); httpSvc != nil {
		rp.metrics = httpSvc.Metrics
		rp.serviceName = httpSvc.name
	}

	return rp
}

// retryProvider retries calls to the wrapped service. A RetryConfig sets maxRetries and leaves policy nil;
// a RetryPolicy sets policy.
type retryProvider struct {
	maxRetries  int
	policy      *retrySettings
	wait        func(context.Context, time.Duration) error
	metrics     Metrics
	logger      debugLogger
	serviceName string
	HTTP
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

// doWithRetry makes the request under the RetryPolicy when there is one, and under RetryConfig otherwise.
func (rp *retryProvider) doWithRetry(ctx context.Context, method string, headers map[string]string,
	reqFunc func(context.Context) (*http.Response, error)) (*http.Response, error) {
	if rp.policy != nil {
		return rp.doWithPolicy(ctx, method, headers, reqFunc)
	}

	return rp.doWithRetryConfig(ctx, func() (*http.Response, error) { return reqFunc(ctx) })
}

// doWithRetryConfig makes the request, then retries network errors and responses above 500 up to maxRetries times.
//
// Retries stop as soon as the caller's context is done. Every further attempt would fail the same way
// without reaching the network, and would only be logged, recorded as another failed outbound call, and
// counted by an inner circuit breaker. The result returned is the result the final retry would have
// returned.
//
// The context is checked on both sides of the attempt, because the two cases are not the same one.
// Checking only before it -- which is all this did at first -- misses the common case: a deadline that
// passes while an attempt is already on the wire. That attempt returns the context's error, the loop
// then reads a context that was live when it was sampled, and a whole extra request goes out.
func (rp *retryProvider) doWithRetryConfig(ctx context.Context, reqFunc func() (*http.Response, error)) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for i := 0; i <= rp.maxRetries; i++ {
		// Before: this attempt is the last, but it is still made. Breaking here instead would return the
		// previous attempt's result, which changes the error the caller sees.
		ctxDone := ctx.Err() != nil

		resp, err = reqFunc()
		if err == nil && resp.StatusCode <= 500 {
			return resp, nil
		}

		if i > 0 && rp.metrics != nil {
			rp.metrics.IncrementCounter(context.Background(), retryCountMetric, "service", rp.serviceName)
		}

		if ctxDone || contextEndedTheAttempt(ctx, err) || i == rp.maxRetries {
			break
		}

		// The attempt is being replaced, so nobody reads this response: drain it so its connection goes
		// back to the pool instead of being abandoned.
		drainAndCloseResponse(resp)
	}

	return resp, err
}

// doWithPolicy makes the request, then retries it as the RetryPolicy allows. Whenever it stops early --
// the request may not be retried, the budget is spent, or the wait would outlast the caller's context
// -- it returns the last attempt's result as it is, with the body unread.
//
// The caller's context is checked on both sides of each attempt, as in doWithRetryConfig.
func (rp *retryProvider) doWithPolicy(ctx context.Context, method string, headers map[string]string,
	reqFunc func(context.Context) (*http.Response, error)) (*http.Response, error) {
	p := rp.policy
	canRetry := p.canRetry(method, headers)

	if p.budget != nil {
		p.budget.deposit()
	}

	for retry := 1; ; retry++ {
		ctxDone := ctx.Err() != nil

		terminal := !canRetry || retry >= p.maxAttempts

		resp, timedOut, err := attemptWithTimeout(ctx, p.attemptTimeout(terminal), reqFunc)

		if terminal || ctxDone || contextEndedTheAttempt(ctx, err) {
			return resp, err
		}

		reason, wait, ok := rp.planRetry(ctx, retry, resp, err, timedOut)
		if !ok {
			return resp, err
		}

		// The connection goes back to the pool during the wait; the response stays readable in case no
		// retry follows.
		detachBody(resp)

		// The budget is spent, and the retry recorded, only once the wait is over and the retry is made.
		if rp.wait(ctx, wait) != nil || !rp.budgetAllows(true) {
			return resp, err
		}

		rp.recordRetry(ctx, retry, reason, wait)

		// The attempt is being replaced, so nobody reads this response: drain it so its connection goes
		// back to the pool instead of being abandoned.
		drainAndCloseResponse(resp)
	}
}

// planRetry decides whether retry number retry is made after an attempt that returned resp and err, and
// if so, the reason to record and the wait before it. A retry is not made if the result is not retryable,
// the wait would outlast the caller's deadline, or the budget is spent.
func (rp *retryProvider) planRetry(ctx context.Context, retry int, resp *http.Response, err error,
	timedOut bool) (reason string, wait time.Duration, ok bool) {
	p := rp.policy

	reason, ok = p.retryReason(resp, err, timedOut)
	if !ok {
		return "", 0, false
	}

	// Stop if the caller's deadline leaves no room for the wait, or -- when attempts are bounded -- for
	// the wait plus one full attempt. A retry sent with a few milliseconds left would only swap the
	// response in hand for the caller's deadline error.
	wait = p.waitBefore(retry, resp, err)
	if deadline, has := ctx.Deadline(); has && time.Until(deadline) < wait+p.perAttemptTimeout {
		return "", 0, false
	}

	// Checked before the wait too, so a call does not wait for a retry the budget already refuses.
	if !rp.budgetAllows(false) {
		return "", 0, false
	}

	return reason, wait, true
}

// budgetAllows reports whether the budget allows a retry, and spends it if withdraw is set. A refusal is
// counted in app_http_retry_budget_exhausted_total.
func (rp *retryProvider) budgetAllows(withdraw bool) bool {
	b := rp.policy.budget
	if b == nil {
		return true
	}

	var allowed bool
	if withdraw {
		allowed = b.tryWithdraw()
	} else {
		allowed = b.allows()
	}

	if !allowed && rp.metrics != nil {
		rp.metrics.IncrementCounter(context.Background(), retryBudgetExhaustedMetric, "service", rp.serviceName)
	}

	return allowed
}

// recordRetry reports that retry number retry is about to be made, after waiting wait.
func (rp *retryProvider) recordRetry(ctx context.Context, retry int, reason string, wait time.Duration) {
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
