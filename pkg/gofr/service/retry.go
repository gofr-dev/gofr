package service

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// discardedBodyLimit bounds the read of a response body that a retry is about to replace. It only has
// to be large enough that an ordinary error body is consumed in full, so the connection goes back to
// the pool; a body larger than this is one where paying for a new connection is the cheaper side.
const discardedBodyLimit = 4 << 10

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

type retryProvider struct {
	maxRetries  int
	metrics     Metrics
	serviceName string
	HTTP
}

func (rp *retryProvider) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response,
	error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.Get(ctx, path, queryParams)
	})
}

func (rp *retryProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	})
}

func (rp *retryProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.Post(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Put(ctx context.Context, api string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.Put(ctx, api, queryParams, body)
	})
}

func (rp *retryProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.Patch(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.Delete(ctx, path, body)
	})
}

func (rp *retryProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.DeleteWithHeaders(ctx, path, body, headers)
	})
}

func (rp *retryProvider) Query(ctx context.Context, path string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.Query(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) QueryWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(ctx, func() (*http.Response, error) {
		return rp.HTTP.QueryWithHeaders(ctx, path, queryParams, body, headers)
	})
}

// doWithRetry makes the request, then retries network errors and responses above 500 up to maxRetries times.
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
func (rp *retryProvider) doWithRetry(ctx context.Context, reqFunc func() (*http.Response, error)) (*http.Response, error) {
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
			rp.metrics.IncrementCounter(context.Background(), "app_http_retry_count", "service", rp.serviceName)
		}

		if ctxDone || contextEndedTheAttempt(ctx, err) || i == rp.maxRetries {
			break
		}

		drainAndClose(resp)
	}

	return resp, err
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

// drainAndClose discards a response a retry is about to replace.
//
// Draining before closing is what returns the connection to the pool: net/http only reuses a
// connection whose body reached EOF, so a Close on an unread body makes it abandon the connection and
// the next attempt opens a fresh one. The read is bounded because a >500 body is not necessarily
// small, and anything past the limit is worth a new connection rather than an unbounded read.
func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	_, _ = io.CopyN(io.Discard, resp.Body, discardedBodyLimit)
	_ = resp.Body.Close()
}
