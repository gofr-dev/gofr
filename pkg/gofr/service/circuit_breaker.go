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
	metrics      Metrics
	serviceName  string

	// healthEndpoint is the custom endpoint to use for health checks during circuit recovery.
	healthEndpoint string
	// healthTimeout is the timeout in seconds for health check requests.
	healthTimeout int

	HTTP
}

// NewCircuitBreaker creates a new circuitBreaker instance based on the provided config.
//
//nolint:revive // Allow returning unexported types as intended.
func NewCircuitBreaker(config CircuitBreakerConfig, h HTTP) *circuitBreaker {
	cb := &circuitBreaker{
		state:         ClosedState,
		threshold:     config.Threshold,
		interval:      config.Interval,
		healthTimeout: defaultTimeout,
		HTTP:          h,
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
	if err != nil || (result != nil && result.StatusCode > 500) {
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
// If a custom healthEndpoint is configured, it uses that endpoint.
// Otherwise, it falls back to the default /.well-known/alive endpoint.
func (cb *circuitBreaker) healthCheck(ctx context.Context) bool {
	var resp *Health

	if cb.healthEndpoint != "" {
		// Use the custom health endpoint configured in CircuitBreakerConfig
		resp = cb.HTTP.getHealthResponseForEndpoint(ctx, cb.healthEndpoint, cb.healthTimeout)
	} else {
		// Fall back to the default health check (/.well-known/alive)
		resp = cb.HTTP.HealthCheck(ctx)
	}

	return resp.Status == serviceUp
}

// startHealthChecks initiates periodic health checks.
func (cb *circuitBreaker) startHealthChecks() {
	ticker := time.NewTicker(cb.interval)

	for range ticker.C {
		if cb.isOpen() {
			go func() {
				if cb.healthCheck(context.TODO()) {
					cb.mu.Lock()
					defer cb.mu.Unlock()

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

	if cb.metrics != nil {
		cb.metrics.SetGauge("app_http_circuit_breaker_state", 1, "service", cb.serviceName)
	}
}

// resetCircuit transitions the circuit breaker to the closed state.
func (cb *circuitBreaker) resetCircuit() {
	cb.state = ClosedState
	cb.failureCount = 0

	if cb.metrics != nil {
		cb.metrics.SetGauge("app_http_circuit_breaker_state", 0, "service", cb.serviceName)
	}
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

// setHealthConfig updates the circuit breaker's health endpoint and timeout.
// This is called by HealthConfig.AddOption when it wraps a circuit breaker,
// ensuring the circuit breaker uses the same health endpoint for recovery checks.
func (cb *circuitBreaker) setHealthConfig(endpoint string, timeout int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.healthEndpoint = endpoint
	cb.healthTimeout = timeout
}

func (cb *CircuitBreakerConfig) AddOption(h HTTP) HTTP {
	circuitBreaker := NewCircuitBreaker(*cb, h)

	if httpSvc := extractHTTPService(h); httpSvc != nil {
		circuitBreaker.metrics = httpSvc.Metrics
		circuitBreaker.serviceName = httpSvc.name

		if circuitBreaker.metrics != nil {
			registerGauge(circuitBreaker.metrics, "app_http_circuit_breaker_state",
				"Current state of the circuit breaker (0 for Closed, 1 for Open)")

			// Initialize the gauge to 0 (Closed)
			circuitBreaker.metrics.SetGauge("app_http_circuit_breaker_state", 0, "service", circuitBreaker.serviceName)
		}
	}

	return circuitBreaker
}

func (cb *circuitBreaker) tryCircuitRecovery() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == ClosedState {
		return true
	}

	if time.Since(cb.lastChecked) > cb.interval {
		// Update lastChecked to prevent busy loop of health checks from other requests
		cb.lastChecked = time.Now()

		if cb.healthCheck(context.TODO()) {
			cb.resetCircuit()
			return true
		}
	}

	return false
}

func (*circuitBreaker) handleCircuitBreakerResult(result any, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}

	response, ok := result.(*http.Response)
	if !ok {
		return nil, ErrUnexpectedCircuitBreakerResultType
	}

	return response, nil
}

func (cb *circuitBreaker) doRequest(ctx context.Context, method, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	if cb.isOpen() {
		if !cb.tryCircuitRecovery() {
			return nil, ErrCircuitOpen
		}
	}

	var result any

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

func (cb *circuitBreaker) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodGet, path, queryParams, nil, headers)
}

// PostWithHeaders is a wrapper for doRequest with the POST method and headers.
func (cb *circuitBreaker) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPost, path, queryParams, body, headers)
}

// PatchWithHeaders is a wrapper for doRequest with the PATCH method and headers.
func (cb *circuitBreaker) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPatch, path, queryParams, body, headers)
}

// PutWithHeaders is a wrapper for doRequest with the PUT method and headers.
func (cb *circuitBreaker) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPut, path, queryParams, body, headers)
}

// DeleteWithHeaders is a wrapper for doRequest with the DELETE method and headers.
func (cb *circuitBreaker) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return cb.doRequest(ctx, http.MethodDelete, path, nil, body, headers)
}

func (cb *circuitBreaker) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodGet, path, queryParams, nil, nil)
}

// Post is a wrapper for doRequest with the POST method and headers.
func (cb *circuitBreaker) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPost, path, queryParams, body, nil)
}

// Patch is a wrapper for doRequest with the PATCH method and headers.
func (cb *circuitBreaker) Patch(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPatch, path, queryParams, body, nil)
}

// Put is a wrapper for doRequest with the PUT method and headers.
func (cb *circuitBreaker) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, http.MethodPut, path, queryParams, body, nil)
}

// Delete is a wrapper for doRequest with the DELETE method and headers.
func (cb *circuitBreaker) Delete(ctx context.Context, path string, body []byte) (
	*http.Response, error) {
	return cb.doRequest(ctx, http.MethodDelete, path, nil, body, nil)
}
