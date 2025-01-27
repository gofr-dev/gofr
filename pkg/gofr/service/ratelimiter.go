package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	shutdownTimeout = 100 * time.Millisecond
)

var (
	ErrQueueFull      = errors.New("request queue is full")
	ErrShuttingDown   = errors.New("rate limiter is shutting down")
	ErrPanicRecovered = errors.New("panic recovered in request processing")
	ErrShutdownFailed = errors.New("shutdown timeout")
)

type RateLimiterConfig struct {
	Limit, MaxQueue int
	Duration        time.Duration
}

func (r *RateLimiterConfig) AddOption(h HTTP) HTTP {
	rl := NewRateLimiter(r.Limit, r.Duration, r.MaxQueue)
	rl.HTTP = h

	return rl
}

type RateLimiter struct {
	HTTP
	limiter        *rate.Limiter
	requests       chan requestWrapper
	maxQueue       int
	mutex          sync.Mutex
	done           chan struct{}
	wg             sync.WaitGroup
	isShuttingDown bool
}

type requestWrapper struct {
	execute func() (*http.Response, error)
	respCh  chan *requestResponse
	ctx     context.Context
	cancel  context.CancelFunc
}

type requestResponse struct {
	response *http.Response
	err      error
}

func NewRateLimiter(limit int, window time.Duration, maxQueue int) *RateLimiter {
	if maxQueue <= 0 {
		maxQueue = 1000
	}

	lim := rate.NewLimiter(rate.Every(window/time.Duration(limit)), limit)
	rl := &RateLimiter{
		limiter:  lim,
		requests: make(chan requestWrapper, maxQueue),
		maxQueue: maxQueue,
		done:     make(chan struct{}),
	}

	rl.Start()

	return rl
}

func handleContextDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		span := trace.SpanFromContext(ctx)
		spanContext := span.SpanContext()
		traceID := spanContext.TraceID().String()
		spanID := spanContext.SpanID().String()

		newCtx := context.Background()
		if spanContext.IsValid() {
			newCtx = trace.ContextWithSpanContext(newCtx, spanContext)

			fmt.Printf("Request with TraceID: %s, SpanID: %s\n", traceID, spanID)
		}

		return context.WithCancel(newCtx)
	}

	return context.WithCancel(ctx)
}

func (r *RateLimiter) Shutdown(ctx context.Context) error {
	r.mutex.Lock()
	if r.isShuttingDown {
		r.mutex.Unlock()
		return ErrShuttingDown
	}

	r.isShuttingDown = true
	r.mutex.Unlock()

	close(r.done)

	completed := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(completed)
	}()

	select {
	case <-ctx.Done():
		select {
		case <-completed:
			return nil
		case <-time.After(shutdownTimeout):
			return fmt.Errorf("%w: %w", ErrShutdownFailed, ctx.Err())
		}
	case <-completed:
		return nil
	}
}

func (r *RateLimiter) handleResponse(ctx context.Context, respCh chan *requestResponse,
	reqCancel context.CancelFunc) (*http.Response, error) {
	select {
	case resp := <-respCh:
		reqCancel()

		if resp.err != nil {
			span := trace.SpanFromContext(ctx)
			traceID := span.SpanContext().TraceID().String()

			return nil, fmt.Errorf("request failed for TraceID %s: %w", traceID, resp.err)
		}

		return resp.response, resp.err
	case <-ctx.Done():
		reqCancel()

		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()

		return nil, fmt.Errorf("context canceled while waiting for request with TraceID %s: %w", traceID, ctx.Err())
	case <-r.done:
		reqCancel()

		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()

		return nil, fmt.Errorf("rate limiter shutting down while waiting for request with TraceID %s: %w", traceID, ErrShuttingDown)
	}
}

func (r *RateLimiter) enqueueRequest(ctx context.Context, execute func() (*http.Response, error)) (*http.Response, error) {
	ctx, cancel := handleContextDeadline(ctx)
	defer cancel()

	r.mutex.Lock()
	if r.isShuttingDown {
		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		r.mutex.Unlock()

		return nil, fmt.Errorf("rate limiter shutting down for request with TraceID %s: %w", traceID, ErrShuttingDown)
	}

	queueLen := len(r.requests)
	if queueLen >= r.maxQueue {
		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		r.mutex.Unlock()

		return nil, fmt.Errorf("%w for request with TraceID %s (max size: %d)", ErrQueueFull, traceID, r.maxQueue)
	}
	r.mutex.Unlock()

	respCh := make(chan *requestResponse, 1)
	reqCtx, reqCancel := context.WithCancel(ctx)

	r.wg.Add(1)

	req := requestWrapper{
		execute: execute,
		respCh:  respCh,
		ctx:     reqCtx,
		cancel:  reqCancel,
	}

	select {
	case r.requests <- req:
		if queueLen > 0 {
			span := trace.SpanFromContext(ctx)
			traceID := span.SpanContext().TraceID().String()
			fmt.Printf("Request queued with TraceID: %s. Queue length: %d\n", traceID, queueLen)
		}

		return r.handleResponse(ctx, respCh, reqCancel)

	case <-ctx.Done():
		r.wg.Done()
		reqCancel()

		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()

		return nil, fmt.Errorf("context canceled while enqueueing for request with TraceID %s: %w", traceID, ctx.Err())

	case <-r.done:
		r.wg.Done()
		reqCancel()

		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()

		return nil, fmt.Errorf("rate limiter shutting down for request with TraceID %s: %w", traceID, ErrShuttingDown)
	}
}

func (r *RateLimiter) Start() {
	go func() {
		for {
			select {
			case <-r.done:
				r.drainQueue()
				return
			case req := <-r.requests:
				go r.processRequest(req)
			}
		}
	}()
}

func (r *RateLimiter) drainQueue() {
	for {
		select {
		case req := <-r.requests:
			go r.processRequest(req)
		default:
			return
		}
	}
}

func (r *RateLimiter) processRequest(req requestWrapper) {
	defer func() {
		if rec := recover(); rec != nil {
			req.respCh <- &requestResponse{
				response: nil,
				err:      fmt.Errorf("%w: %v", ErrPanicRecovered, rec),
			}
		}

		r.wg.Done()
		req.cancel()
	}()

	select {
	case <-req.ctx.Done():
		req.respCh <- &requestResponse{nil, fmt.Errorf("request canceled in queue: %w", req.ctx.Err())}
		return
	default:
	}

	if err := r.limiter.Wait(req.ctx); err != nil {
		req.respCh <- &requestResponse{nil, fmt.Errorf("rate limit wait error: %w", err)}
		return
	}

	//nolint:bodyclose // Response body must be closed by the consumer
	resp, err := req.execute()
	if err != nil {
		req.respCh <- &requestResponse{nil, fmt.Errorf("request execution error: %w", err)}
		return
	}

	req.respCh <- &requestResponse{resp, nil}
}

func (r *RateLimiter) Get(ctx context.Context, api string, queryParams map[string]any) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Get(ctx, api, queryParams)
	})
}

func (r *RateLimiter) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	})
}

func (r *RateLimiter) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Post(ctx, path, queryParams, body)
	})
}

func (r *RateLimiter) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (r *RateLimiter) Put(ctx context.Context, api string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Put(ctx, api, queryParams, body)
	})
}

func (r *RateLimiter) PutWithHeaders(ctx context.Context, api string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.PutWithHeaders(ctx, api, queryParams, body, headers)
	})
}

func (r *RateLimiter) Patch(ctx context.Context, api string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Patch(ctx, api, queryParams, body)
	})
}

func (r *RateLimiter) PatchWithHeaders(ctx context.Context, api string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.PatchWithHeaders(ctx, api, queryParams, body, headers)
	})
}

func (r *RateLimiter) Delete(ctx context.Context, api string, body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Delete(ctx, api, body)
	})
}

func (r *RateLimiter) DeleteWithHeaders(ctx context.Context, api string, body []byte, headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.DeleteWithHeaders(ctx, api, body, headers)
	})
}

func APIRateLimit(limit int, duration time.Duration, maxQueue int) *RateLimiterConfig {
	return &RateLimiterConfig{
		Limit:    limit,
		Duration: duration,
		MaxQueue: maxQueue,
	}
}
