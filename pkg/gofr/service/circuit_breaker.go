package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// circuitBreaker states.
const (
	ClosedState = iota
	OpenState
)

var (
	// ErrCircuitOpen indicates that the circuit breaker is open.
	ErrCircuitOpen                        = errors.New("unable to connect to server at host")
	ErrUnexpectedCircuitBreakerResultType = errors.New("unexpected result type from circuit breaker")
)

// CircuitBreakerConfig holds the configuration for the circuitBreaker.
type CircuitBreakerConfig struct {
	Threshold int           // Threshold represents the max no of retry before switching the circuit breaker state.
	Interval  time.Duration // Interval represents the time interval duration between hitting the HealthURL
}

// circuitBreaker represents a circuit breaker implementation.
type circuitBreaker struct {
	mu           sync.RWMutex
	state        int // ClosedState or OpenState
	failureCount int
	threshold    int
	interval     time.Duration
	lastChecked  time.Time

	HTTP
}

// NewCircuitBreaker creates a new circuitBreaker instance based on the provided config.
//
//nolint:revive // We do not want anyone using the circuit breaker without initialization steps.
func NewCircuitBreaker(config CircuitBreakerConfig, h HTTP) *circuitBreaker {
	cb := &circuitBreaker{
		state:     ClosedState,
		threshold: config.Threshold,
		interval:  config.Interval,
		HTTP:      h,
	}

	// Perform asynchronous health checks
	go cb.startHealthChecks()

	return cb
}

// executeWithCircuitBreaker executes the given function with circuit breaker protection.
func (cb *circuitBreaker) executeWithCircuitBreaker(ctx context.Context, f func(ctx context.Context) (*http.Response,
	error)) (*http.Response, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == OpenState {
		if time.Since(cb.lastChecked) > cb.interval {
			// Check health before potentially closing the circuit
			if cb.healthCheck(ctx) {
				cb.resetCircuit()
				return nil, nil
			}
		}

		return nil, ErrCircuitOpen
	}

	result, err := f(ctx)

	if err != nil {
		cb.handleFailure()
	} else {
		cb.resetFailureCount()
	}

	if cb.failureCount > cb.threshold {
		cb.openCircuit()
		return nil, ErrCircuitOpen
	}

	return result, err
}

// isOpen returns true if the circuit breaker is in the open state.
func (cb *circuitBreaker) isOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.state == OpenState
}

// healthCheck performs the health check for the circuit breaker.
func (cb *circuitBreaker) healthCheck(ctx context.Context) bool {
	resp := cb.HealthCheck(ctx)

	return resp.Status == serviceUp
}

// startHealthChecks initiates periodic health checks.
func (cb *circuitBreaker) startHealthChecks() {
	ticker := time.NewTicker(cb.interval)

	for range ticker.C {
		if cb.isOpen() {
			go func() {
				if cb.healthCheck(context.TODO()) {
					cb.resetCircuit()
				}
			}()
		}
	}
}

// openCircuit transitions the circuit breaker to the open state.
func (cb *circuitBreaker) openCircuit() {
	cb.state = OpenState
	cb.lastChecked = time.Now()
}

// resetCircuit transitions the circuit breaker to the closed state.
func (cb *circuitBreaker) resetCircuit() {
	cb.state = ClosedState
	cb.failureCount = 0
}

// handleFailure increments the failure count and opens the circuit if the threshold is reached.
func (cb *circuitBreaker) handleFailure() {
	cb.failureCount++
	if cb.failureCount > cb.threshold {
		cb.openCircuit()
	}
}

// resetFailureCount resets the failure count to zero.
func (cb *circuitBreaker) resetFailureCount() {
	cb.failureCount = 0
}

func (cb *CircuitBreakerConfig) AddOption(h HTTP) HTTP {
	return NewCircuitBreaker(*cb, h)
}

func (cb *circuitBreaker) tryCircuitRecovery() bool {
	if time.Since(cb.lastChecked) > cb.interval && cb.healthCheck(context.TODO()) {
		cb.resetCircuit()
		return true
	}

	return false
}

func (*circuitBreaker) handleCircuitBreakerResult(result interface{}, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}

	response, ok := result.(*http.Response)
	if !ok {
		return nil, ErrUnexpectedCircuitBreakerResultType
	}

	return response, nil
}

func (cb *circuitBreaker) doRequest(ctx context.Context, method, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	if cb.isOpen() {
		if !cb.tryCircuitRecovery() {
			return nil, ErrCircuitOpen
		}
	}

	var result interface{}

	var err error

	switch method {
	case http.MethodGet:
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
		})
	case http.MethodPost:
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
		})
	case http.MethodPatch:
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
		})
	case http.MethodPut:
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
		})
	case http.MethodDelete:
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTP.DeleteWithHeaders(ctx, path, body, headers)
		})
	}

	resp, err := cb.handleCircuitBreakerResult(result, err)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (cb *circuitBreaker) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

// PostWithHeaders is a wrapper for doRequest with the POST method and headers.
func (cb *circuitBreaker) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

// PatchWithHeaders is a wrapper for doRequest with the PATCH method and headers.
func (cb *circuitBreaker) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

// PutWithHeaders is a wrapper for doRequest with the PUT method and headers.
func (cb *circuitBreaker) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

// DeleteWithHeaders is a wrapper for doRequest with the DELETE method and headers.
func (cb *circuitBreaker) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return cb.doRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (cb *circuitBreaker) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodGet, path, queryParams, nil, nil)
}

// Post is a wrapper for doRequest with the POST method and headers.
func (cb *circuitBreaker) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPost, path, queryParams, body, nil)
}

// Patch is a wrapper for doRequest with the PATCH method and headers.
func (cb *circuitBreaker) Patch(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPatch, path, queryParams, body, nil)
}

// Put is a wrapper for doRequest with the PUT method and headers.
func (cb *circuitBreaker) Put(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPut, path, queryParams, body, nil)
}

// Delete is a wrapper for doRequest with the DELETE method and headers.
func (cb *circuitBreaker) Delete(ctx context.Context, path string, body []byte) (
	*http.Response, error) {
	return cb.doRequest(ctx, http.MethodDelete, path, nil, body, nil)
}
