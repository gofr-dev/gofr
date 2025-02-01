package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

var (
	errQueueFull       = errors.New("request queue is full")
	errShuttingDown    = errors.New("rate limiter is shutting down")
	errPanicRecovered  = errors.New("panic recovered in request processing")
	errRequestComplete = errors.New("request completed")
)

// RateLimiterConfig holds configuration for creating a new rate limiter.
type RateLimiterConfig struct {
	Limit, MaxQueue int
	Duration        time.Duration
}

// AddOption creates a new rate limiter with the config settings and wraps the provided HTTP client.
func (r *RateLimiterConfig) AddOption(h HTTP) HTTP {
	switch v := h.(type) {
	case *httpService:
		rl := NewRateLimiter(context.Background(), r.Limit, r.Duration, r.MaxQueue)
		rl.HTTP = h
		rl.Logger = v.Logger

		return rl

	case *RateLimiter:
		return h
	default:
		rl := NewRateLimiter(context.Background(), r.Limit, r.Duration, r.MaxQueue)
		rl.HTTP = h

		return rl
	}
}

type RateLimiter struct {
	HTTP
	limiter        *rate.Limiter
	requests       chan requestWrapper
	maxQueue       int
	mutex          sync.Mutex
	wg             sync.WaitGroup
	isShuttingDown bool
	currentQueue   atomic.Int32
	Logger
}

type requestWrapper struct {
	execute func() (*http.Response, error)
	respCh  chan *requestResponse
	ctx     context.Context
	cancel  context.CancelCauseFunc
}

type requestResponse struct {
	response *http.Response
	err      error
}

// NewRateLimiter creates a rate limiter.
func NewRateLimiter(ctx context.Context, limit int, window time.Duration, maxQueue int) *RateLimiter {
	if maxQueue <= 0 {
		maxQueue = 1000
	}

	lim := rate.NewLimiter(rate.Every(window/time.Duration(limit)), limit)
	rl := &RateLimiter{
		limiter:  lim,
		requests: make(chan requestWrapper, maxQueue),
		maxQueue: maxQueue,
	}
	rl.currentQueue.Store(0)

	rl.Start(ctx)

	return rl
}

func handleContextDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		newCtx := context.WithoutCancel(ctx)
		ctx = newCtx
	}

	return context.WithCancel(ctx)
}

// enqueueRequest adds a request to the rate limiter queue and waits for its response.
func (r *RateLimiter) enqueueRequest(ctx context.Context, execute func() (*http.Response, error)) (*http.Response, error) {
	ctx, cancel := handleContextDeadline(ctx)
	defer cancel()

	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	startTime := time.Now()

	r.mutex.Lock()
	if r.isShuttingDown {
		r.mutex.Unlock()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusServiceUnavailable,
				HTTPMethod:    "REQUEST",
				URI:           "rate_limiter",
			},
			ErrorMessage: "rate limiter shutting down",
		})

		return nil, fmt.Errorf("rate limiter shutting down for request with TraceID %s: %w", traceID, errShuttingDown)
	}

	currentQueueSize := r.currentQueue.Load()
	//nolint:gosec // Acceptable int to int32 conversion as maxQueue is bounded
	if currentQueueSize >= int32(r.maxQueue) {
		r.mutex.Unlock()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusServiceUnavailable,
				HTTPMethod:    "REQUEST",
				URI:           "rate_limiter",
			},
			ErrorMessage: fmt.Sprintf("queue full (max size: %d, current size: %d)", r.maxQueue, currentQueueSize),
		})

		return nil, fmt.Errorf("%w for request with TraceID %s (max size: %d, current size: %d)",
			errQueueFull, traceID, r.maxQueue, currentQueueSize)
	}
	r.mutex.Unlock()

	respCh := make(chan *requestResponse, 1)
	reqCtx, reqCancel := context.WithCancelCause(ctx)

	if !r.currentQueue.CompareAndSwap(currentQueueSize, currentQueueSize+1) {
		reqCancel(errQueueFull)
		return nil, fmt.Errorf("%w for request with TraceID %s (queue size changed)", errQueueFull, traceID)
	}

	r.wg.Add(1)

	req := requestWrapper{
		execute: execute,
		respCh:  respCh,
		ctx:     reqCtx,
		cancel:  reqCancel,
	}

	select {
	case r.requests <- req:
		r.Log(&Log{
			Timestamp:     time.Now(),
			CorrelationID: traceID,
			ResponseTime:  time.Since(startTime).Microseconds(),
			ResponseCode:  http.StatusAccepted,
			HTTPMethod:    "REQUEST",
			URI:           "rate_limiter",
		})

		return r.handleResponse(ctx, respCh, reqCancel)

	case <-ctx.Done():
		cause := context.Cause(ctx)

		r.currentQueue.Add(-1)
		r.wg.Done()
		reqCancel(cause)
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusGatewayTimeout,
				HTTPMethod:    "REQUEST",
				URI:           "rate_limiter",
			},
			ErrorMessage: fmt.Sprintf("context canceled while enqueueing: %v", cause),
		})

		return nil, fmt.Errorf("context canceled while enqueueing with TraceID %s: %w", traceID, cause)
	}
}

func (r *RateLimiter) handleResponse(ctx context.Context, respCh chan *requestResponse,
	reqCancel context.CancelCauseFunc) (*http.Response, error) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	startTime := time.Now()

	select {
	case resp := <-respCh:
		reqCancel(errRequestComplete)

		if resp.err != nil {
			r.Log(&ErrorLog{
				Log: &Log{
					Timestamp:     time.Now(),
					CorrelationID: traceID,
					ResponseTime:  time.Since(startTime).Microseconds(),
					ResponseCode:  http.StatusInternalServerError,
					HTTPMethod:    "REQUEST",
					URI:           "rate_limiter",
				},
				ErrorMessage: fmt.Sprintf("request failed: %v", resp.err),
			})

			return nil, fmt.Errorf("request failed for TraceID %s: %w", traceID, resp.err)
		}

		return resp.response, resp.err

	case <-ctx.Done():
		cause := context.Cause(ctx)
		reqCancel(cause)
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusGatewayTimeout,
				HTTPMethod:    "REQUEST",
				URI:           "rate_limiter",
			},
			ErrorMessage: fmt.Sprintf("context canceled while waiting: %v", cause),
		})

		return nil, fmt.Errorf("context canceled while waiting for request with TraceID %s: %w", traceID, cause)
	}
}

// Start begins processing requests from the queue.
func (r *RateLimiter) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				r.drainQueue()
				return
			case req := <-r.requests:
				go r.processRequest(req)
			}
		}
	}()
}

// drainQueue processes remaining requests during shutdown.
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

// processRequest executes a single request with rate limiting.
func (r *RateLimiter) processRequest(req requestWrapper) {
	defer func() {
		r.currentQueue.Add(-1)
		r.wg.Done()
		req.cancel(errRequestComplete) // Add error cause here
	}()

	startTime := time.Now()
	span := trace.SpanFromContext(req.ctx)
	traceID := span.SpanContext().TraceID().String()

	defer func() {
		if rec := recover(); rec != nil {
			r.Log(&ErrorLog{
				Log: &Log{
					Timestamp:     time.Now(),
					CorrelationID: traceID,
					ResponseTime:  time.Since(startTime).Microseconds(),
					ResponseCode:  http.StatusInternalServerError,
				},
				ErrorMessage: fmt.Sprintf("panic recovered in request processing: %v", rec),
			})
			req.respCh <- &requestResponse{
				response: nil,
				err:      fmt.Errorf("%w: %v", errPanicRecovered, rec),
			}
		}
	}()

	select {
	case <-req.ctx.Done():
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusGatewayTimeout,
			},
			ErrorMessage: fmt.Sprintf("request canceled in queue: %v", req.ctx.Err()),
		})
		req.respCh <- &requestResponse{nil, fmt.Errorf("request canceled in queue: %w", req.ctx.Err())}

		return
	default:
	}

	if err := r.limiter.Wait(req.ctx); err != nil {
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusTooManyRequests,
			},
			ErrorMessage: fmt.Sprintf("rate limit wait error: %v", err),
		})
		req.respCh <- &requestResponse{nil, fmt.Errorf("rate limit wait error: %w", err)}

		return
	}

	//nolint:bodyclose // Response body is closed by the caller
	resp, err := req.execute()
	if err != nil {
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusInternalServerError,
			},
			ErrorMessage: fmt.Sprintf("request execution error: %v", err),
		})
		req.respCh <- &requestResponse{nil, fmt.Errorf("request execution error: %w", err)}

		return
	}

	r.Log(&Log{
		Timestamp:     time.Now(),
		CorrelationID: traceID,
		ResponseTime:  time.Since(startTime).Microseconds(),
		ResponseCode:  resp.StatusCode,
	})

	req.respCh <- &requestResponse{resp, nil}
}

// Get performs a rate-limited GET request.
func (r *RateLimiter) Get(ctx context.Context, api string, queryParams map[string]any) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Get(ctx, api, queryParams)
	})
}

// GetWithHeaders performs a rate-limited GET request with custom headers.
func (r *RateLimiter) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	})
}

// Post performs a rate-limited POST request.
func (r *RateLimiter) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Post(ctx, path, queryParams, body)
	})
}

// PostWithHeaders performs a rate-limited POST request with custom headers.
func (r *RateLimiter) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
	})
}

// Put performs a rate-limited PUT request.
func (r *RateLimiter) Put(ctx context.Context, api string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Put(ctx, api, queryParams, body)
	})
}

// PutWithHeaders performs a rate-limited PUT request with custom headers.
func (r *RateLimiter) PutWithHeaders(ctx context.Context, api string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.PutWithHeaders(ctx, api, queryParams, body, headers)
	})
}

// Patch performs a rate-limited PATCH request.
func (r *RateLimiter) Patch(ctx context.Context, api string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Patch(ctx, api, queryParams, body)
	})
}

// PatchWithHeaders performs a rate-limited PATCH request with custom headers.
func (r *RateLimiter) PatchWithHeaders(ctx context.Context, api string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.PatchWithHeaders(ctx, api, queryParams, body, headers)
	})
}

// Delete performs a rate-limited DELETE request.
func (r *RateLimiter) Delete(ctx context.Context, api string, body []byte) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.Delete(ctx, api, body)
	})
}

// DeleteWithHeaders performs a rate-limited DELETE request with custom headers.
func (r *RateLimiter) DeleteWithHeaders(ctx context.Context, api string, body []byte, headers map[string]string) (*http.Response, error) {
	return r.enqueueRequest(ctx, func() (*http.Response, error) {
		return r.HTTP.DeleteWithHeaders(ctx, api, body, headers)
	})
}

// APIRateLimit creates a rate limiter config.
func APIRateLimit(limit int, duration time.Duration, maxQueue int) *RateLimiterConfig {
	return &RateLimiterConfig{
		Limit:    limit,
		Duration: duration,
		MaxQueue: maxQueue,
	}
}
