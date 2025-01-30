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

// RateLimiterConfig holds configuration for creating a new rate limiter.
type RateLimiterConfig struct {
	Limit, MaxQueue int
	Duration        time.Duration
}

// AddOption creates a new rate limiter with the config settings and wraps the provided HTTP client.
func (r *RateLimiterConfig) AddOption(h HTTP) HTTP {
	if hs, ok := h.(*httpService); ok {
		rl := NewRateLimiter(r.Limit, r.Duration, r.MaxQueue)
		rl.HTTP = h
		rl.Logger = hs.Logger
		return rl
	}

	if _, ok := h.(*RateLimiter); ok {
		return h
	}

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
	Logger
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

// NewRateLimiter creates a rate limiter.
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

// handleContextDeadline ensures context has proper tracing setup and cancellation.
func handleContextDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		span := trace.SpanFromContext(ctx)
		spanContext := span.SpanContext()
		newCtx := context.Background()
		if spanContext.IsValid() {
			newCtx = trace.ContextWithSpanContext(newCtx, spanContext)
		}
		return context.WithCancel(newCtx)
	}
	return context.WithCancel(ctx)
}

// Shutdown gracefully shuts down the rate limiter.
func (r *RateLimiter) Shutdown(ctx context.Context) error {
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
				HTTPMethod:    "SHUTDOWN",
			},
			ErrorMessage: "rate limiter already shutting down",
		})
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
			r.Log(&Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusOK,
				HTTPMethod:    "SHUTDOWN",
			})
			return nil
		case <-time.After(shutdownTimeout):
			r.Log(&ErrorLog{
				Log: &Log{
					Timestamp:     time.Now(),
					CorrelationID: traceID,
					ResponseTime:  time.Since(startTime).Microseconds(),
					ResponseCode:  http.StatusGatewayTimeout,
					HTTPMethod:    "SHUTDOWN",
				},
				ErrorMessage: fmt.Sprintf("shutdown failed: timeout after %v", shutdownTimeout),
			})
			return fmt.Errorf("%w: %w", ErrShutdownFailed, ctx.Err())
		}
	case <-completed:
		r.Log(&Log{
			Timestamp:     time.Now(),
			CorrelationID: traceID,
			ResponseTime:  time.Since(startTime).Microseconds(),
			ResponseCode:  http.StatusOK,
			HTTPMethod:    "SHUTDOWN",
		})
		return nil
	}
}

// handleResponse processes the response from the request channel, handling cancellation and errors.
func (r *RateLimiter) handleResponse(ctx context.Context, respCh chan *requestResponse,
	reqCancel context.CancelFunc) (*http.Response, error) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	startTime := time.Now()

	select {
	case resp := <-respCh:
		reqCancel()
		if resp.err != nil {
			r.Log(&ErrorLog{
				Log: &Log{
					Timestamp:     time.Now(),
					CorrelationID: traceID,
					ResponseTime:  time.Since(startTime).Microseconds(),
					ResponseCode:  http.StatusInternalServerError,
				},
				ErrorMessage: fmt.Sprintf("request failed: %v", resp.err),
			})
			return nil, fmt.Errorf("request failed for TraceID %s: %w", traceID, resp.err)
		}
		return resp.response, resp.err

	case <-ctx.Done():
		reqCancel()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusGatewayTimeout,
			},
			ErrorMessage: fmt.Sprintf("context canceled while waiting: %v", ctx.Err()),
		})
		return nil, fmt.Errorf("context canceled while waiting for request with TraceID %s: %w", traceID, ctx.Err())

	case <-r.done:
		reqCancel()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusServiceUnavailable,
			},
			ErrorMessage: "rate limiter shutting down while waiting for response",
		})
		return nil, fmt.Errorf("rate limiter shutting down while waiting for request with TraceID %s: %w", traceID, ErrShuttingDown)
	}
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
			},
			ErrorMessage: "rate limiter shutting down",
		})
		return nil, fmt.Errorf("rate limiter shutting down for request with TraceID %s: %w", traceID, ErrShuttingDown)
	}

	queueLen := len(r.requests)
	if queueLen >= r.maxQueue {
		r.mutex.Unlock()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusServiceUnavailable,
			},
			ErrorMessage: fmt.Sprintf("queue full (max size: %d)", r.maxQueue),
		})
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
			r.Log(&Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusAccepted,
			})
		}
		return r.handleResponse(ctx, respCh, reqCancel)

	case <-ctx.Done():
		r.wg.Done()
		reqCancel()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusGatewayTimeout,
			},
			ErrorMessage: fmt.Sprintf("context canceled while enqueueing: %v", ctx.Err()),
		})
		return nil, fmt.Errorf("context canceled while enqueueing for request with TraceID %s: %w", traceID, ctx.Err())

	case <-r.done:
		r.wg.Done()
		reqCancel()
		r.Log(&ErrorLog{
			Log: &Log{
				Timestamp:     time.Now(),
				CorrelationID: traceID,
				ResponseTime:  time.Since(startTime).Microseconds(),
				ResponseCode:  http.StatusServiceUnavailable,
			},
			ErrorMessage: "rate limiter shutting down while enqueueing request",
		})
		return nil, fmt.Errorf("rate limiter shutting down for request with TraceID %s: %w", traceID, ErrShuttingDown)
	}
}

// Start begins processing requests from the queue.
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
				err:      fmt.Errorf("%w: %v", ErrPanicRecovered, rec),
			}
		}
		r.wg.Done()
		req.cancel()
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
