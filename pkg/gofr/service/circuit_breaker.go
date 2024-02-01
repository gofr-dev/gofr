package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// CircuitBreaker states.
const (
	ClosedState = iota
	OpenState
)

var (
	// ErrCircuitOpen indicates that the circuit breaker is open.
	ErrCircuitOpen                        = errors.New("unable to connect to server at host")
	ErrUnexpectedCircuitBreakerResultType = errors.New("unexpected result type from circuit breaker")
)

// CircuitBreakerConfig holds the configuration for the CircuitBreaker.
type CircuitBreakerConfig struct {
	Threshold int           // Threshold represents the max no of retry before switching the circuit breaker state.
	Timeout   time.Duration // Timeout represents the time duration for which circuit breaker maintains it's open state.
	Interval  time.Duration // Interval represents the time interval duration between hitting the HealthURL
}

// CircuitBreaker represents a circuit breaker implementation.
type CircuitBreaker struct {
	mu           sync.RWMutex
	state        int // ClosedState or OpenState
	failureCount int
	threshold    int
	timeout      time.Duration
	interval     time.Duration
	lastChecked  time.Time

	HTTPService
}

// NewCircuitBreaker creates a new CircuitBreaker instance based on the provided config.
func NewCircuitBreaker(config CircuitBreakerConfig, h HTTPService) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:       ClosedState,
		threshold:   config.Threshold,
		timeout:     config.Timeout,
		interval:    config.Interval,
		HTTPService: h,
	}

	// Perform asynchronous health checks
	go cb.startHealthChecks()

	return cb
}

// executeWithCircuitBreaker executes the given function with circuit breaker protection.
func (cb *CircuitBreaker) executeWithCircuitBreaker(ctx context.Context, f func(ctx context.Context) (*http.Response,
	error)) (*http.Response, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == OpenState {
		if time.Since(cb.lastChecked) > cb.timeout {
			// Check health before potentially closing the circuit
			if cb.healthCheck() {
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
func (cb *CircuitBreaker) isOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.state == OpenState
}

// healthCheck performs the health check for the circuit breaker.
func (cb *CircuitBreaker) healthCheck() bool {
	rsp := cb.HTTPService.HealthCheck()
	v := rsp.(*Health)

	return v.Status == serviceUp
}

// startHealthChecks initiates periodic health checks.
func (cb *CircuitBreaker) startHealthChecks() {
	ticker := time.NewTicker(cb.interval)

	for range ticker.C {
		if cb.isOpen() {
			go func() {
				if cb.healthCheck() {
					cb.resetCircuit()
				}
			}()
		}
	}
}

// openCircuit transitions the circuit breaker to the open state.
func (cb *CircuitBreaker) openCircuit() {
	cb.state = OpenState
	cb.lastChecked = time.Now()
}

// resetCircuit transitions the circuit breaker to the closed state.
func (cb *CircuitBreaker) resetCircuit() {
	cb.state = ClosedState
	cb.failureCount = 0
}

// handleFailure increments the failure count and opens the circuit if the threshold is reached.
func (cb *CircuitBreaker) handleFailure() {
	cb.failureCount++
	if cb.failureCount > cb.threshold {
		cb.openCircuit()
	}
}

// resetFailureCount resets the failure count to zero.
func (cb *CircuitBreaker) resetFailureCount() {
	cb.failureCount = 0
}

func (cb *CircuitBreakerConfig) addOption(h HTTPService) HTTPService {
	return NewCircuitBreaker(*cb, h)
}

func (cb *CircuitBreaker) tryCircuitRecovery() bool {
	if time.Since(cb.lastChecked) > cb.timeout && cb.healthCheck() {
		cb.resetCircuit()
		return true
	}

	return false
}

func (cb *CircuitBreaker) handleCircuitBreakerResult(result interface{}, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}

	response, ok := result.(*http.Response)
	if !ok {
		return nil, ErrUnexpectedCircuitBreakerResultType
	}

	return response, nil
}

func (cb *CircuitBreaker) doRequest(ctx context.Context, method, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	if cb.isOpen() {
		if !cb.tryCircuitRecovery() {
			return nil, ErrCircuitOpen
		}
	}

	var result interface{}

	var err error

	switch method {
	case "GET":
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTPService.GetWithHeaders(ctx, path, queryParams, headers)
		})
	case "POST":
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTPService.PostWithHeaders(ctx, path, queryParams, body, headers)
		})
	case "PATCH":
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTPService.PatchWithHeaders(ctx, path, queryParams, body, headers)
		})
	case "PUT":
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTPService.PutWithHeaders(ctx, path, queryParams, body, headers)
		})
	case "DELETE":
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTPService.DeleteWithHeaders(ctx, path, body, headers)
		})
	}

	resp, err := cb.handleCircuitBreakerResult(result, err)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (cb *CircuitBreaker) Get(ctx context.Context, api string, queryParams map[string]interface{}) (*http.Response, error) {
	return cb.doRequest(ctx, "GET", api, queryParams, nil, nil)
}
func (cb *CircuitBreaker) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, "GET", path, queryParams, nil, headers)
}

// Post is a wrapper for doRequest with the POST method.
func (cb *CircuitBreaker) Post(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, "POST", path, queryParams, body, nil)
}

// PostWithHeaders is a wrapper for doRequest with the POST method and headers.
func (cb *CircuitBreaker) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, "POST", path, queryParams, body, headers)
}

// Patch is a wrapper for doRequest with the PATCH method.
func (cb *CircuitBreaker) Patch(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, "PATCH", path, queryParams, body, nil)
}

// PatchWithHeaders is a wrapper for doRequest with the PATCH method and headers.
func (cb *CircuitBreaker) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, "PATCH", path, queryParams, body, headers)
}

// PutWithHeaders is a wrapper for doRequest with the PUT method and headers.
func (cb *CircuitBreaker) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, "PUT", path, queryParams, body, headers)
}

// Delete is a wrapper for doRequest with the DELETE method.
func (cb *CircuitBreaker) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, "DELETE", path, nil, body, nil)
}

// DeleteWithHeaders is a wrapper for doRequest with the DELETE method and headers.
func (cb *CircuitBreaker) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return cb.doRequest(ctx, "DELETE", path, nil, body, headers)
}

func (cb *CircuitBreaker) HealthCheck() interface{} {
	return cb.HTTPService.HealthCheck()
}
