package service

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	// errUnsupportedMethod is returned by doRequest when it is asked to route
	// an HTTP method it does not handle. Unexported because doRequest itself is
	// unexported and all in-tree callers pass a known method constant, so the
	// default branch is unreachable from outside the package — but the branch
	// itself is worth keeping: an earlier revision returned (nil, nil) silently
	// in that spot.
	errUnsupportedMethod = errors.New("unsupported HTTP method for circuit breaker")
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

	HTTP
}

// NewCircuitBreaker creates a new circuitBreaker instance based on the provided config.
//
//nolint:revive // Allow returning unexported types as intended.
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
	cb.mu.RLock()
	isOpen := cb.state == OpenState
	cb.mu.RUnlock()

	if isOpen {
		// Circuit is open - try recovery without holding lock
		if !cb.tryCircuitRecovery() {
			return nil, ErrCircuitOpen
		}
		// Circuit recovered, proceed with request
	}

	result, err := f(ctx)

	// A request its own caller abandoned never got a verdict from the upstream, so it is
	// neither a failure nor a success: counting it would let callers who hang up open the
	// breaker for every other caller of a healthy upstream, and resetting on it would erase
	// a real failure streak.
	if canceledByCaller(ctx, err) {
		return result, err
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil || (result != nil && result.StatusCode > 500) {
		cb.handleFailure()

		if cb.state == OpenState {
			// The caller never receives result on this path, so nothing outside this function can
			// close its body -- and an unread body holds its connection out of the pool for good.
			// The breaker opening is precisely when a service is hammering a struggling upstream,
			// so this is the worst moment to leak a connection per call.
			drainAndCloseResponse(result)

			return nil, ErrCircuitOpen
		}
	} else {
		cb.resetFailureCount()
	}

	return result, err
}

// isOpen returns true if the circuit breaker is in the open state.
func (cb *circuitBreaker) isOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.state == OpenState
}

func (cb *circuitBreaker) healthCheck(ctx context.Context) bool {
	if httpSvc := extractHTTPService(cb.HTTP); httpSvc != nil && httpSvc.healthEndpoint != "" {
		resp := cb.HTTP.getHealthResponseForEndpoint(ctx, httpSvc.healthEndpoint, httpSvc.healthTimeout)

		return resp.Status == serviceUp
	}

	resp := cb.HTTP.HealthCheck(ctx)

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
	wasOpen := cb.state == OpenState
	cb.state = OpenState
	cb.lastChecked = time.Now()

	if cb.metrics != nil {
		cb.metrics.SetGauge("app_http_circuit_breaker_state", 1, "service", cb.serviceName)

		// Only count a real Closed -> Open transition. openCircuit is reached
		// from handleFailure whenever failureCount > threshold, and every
		// concurrent request that passed the isOpen check before the trip
		// falls through to handleFailure and re-enters here — without this
		// guard, a burst of N concurrent failures records N-threshold
		// "openings" for what is a single transition.
		if !wasOpen {
			cb.metrics.IncrementCounter(context.Background(), "app_circuit_open_count", "service", cb.serviceName)
		}
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

// canceledByCaller reports whether err is the caller canceling ctx rather than an outcome
// of the upstream. The transport returns context.Cause(ctx), so a caller that canceled with
// context.WithCancelCause sees its own cause instead of context.Canceled; both are matched.
//
// context.DeadlineExceeded is deliberately not matched: a request that ran out of time is
// evidence of a slow upstream and keeps counting as a failure.
//
// The context.Cause arm is wider than it first looks: an upstream error chain that happened to wrap
// the same sentinel a caller passed to WithCancelCause would go unaccounted. That needs the caller
// to have chosen a cause the upstream also returns, which is not a shape reached by accident, and
// the alternative is dropping WithCancelCause support entirely. Of the two mistakes, treating a real
// cancellation as an upstream failure is the worse one -- it is the bug this function exists to fix.
func canceledByCaller(ctx context.Context, err error) bool {
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		return false
	}

	return errors.Is(err, context.Canceled) || errors.Is(err, context.Cause(ctx))
}

// resetFailureCount resets the failure count to zero.
func (cb *circuitBreaker) resetFailureCount() {
	cb.failureCount = 0
}

func (cb *CircuitBreakerConfig) AddOption(h HTTP) HTTP {
	circuitBreaker := NewCircuitBreaker(*cb, h)

	if httpSvc := extractHTTPService(h); httpSvc != nil {
		circuitBreaker.metrics = httpSvc.Metrics
		circuitBreaker.serviceName = httpSvc.name

		if circuitBreaker.metrics != nil {
			// Initialize the gauge to 0 (Closed) - gauge is already registered in container.go
			circuitBreaker.metrics.SetGauge("app_http_circuit_breaker_state", 0, "service", circuitBreaker.serviceName)
		}
	}

	return circuitBreaker
}

func (cb *circuitBreaker) tryCircuitRecovery() bool {
	cb.mu.Lock()

	if cb.state == ClosedState {
		cb.mu.Unlock()
		return true
	}

	if time.Since(cb.lastChecked) > cb.interval {
		// Update lastChecked to prevent busy loop of health checks from other requests
		cb.lastChecked = time.Now()
		cb.mu.Unlock()

		if cb.healthCheck(context.TODO()) {
			cb.mu.Lock()
			defer cb.mu.Unlock()

			if cb.state == OpenState {
				cb.resetCircuit()
			}

			return true
		}

		return false
	}

	cb.mu.Unlock()

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
	case methodQuery:
		result, err = cb.executeWithCircuitBreaker(ctx, func(ctx context.Context) (*http.Response, error) {
			return cb.HTTP.QueryWithHeaders(ctx, path, queryParams, body, headers)
		})
	default:
		return nil, fmt.Errorf("%w: %q", errUnsupportedMethod, method)
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

// QueryWithHeaders is a wrapper for doRequest with the QUERY method and headers.
func (cb *circuitBreaker) QueryWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return cb.doRequest(ctx, methodQuery, path, queryParams, body, headers)
}

// Query is a wrapper for doRequest with the QUERY method.
func (cb *circuitBreaker) Query(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return cb.doRequest(ctx, methodQuery, path, queryParams, body, nil)
}

// discardedBodyLimit bounds the read of a response body nobody will consume. It only has to be large
// enough that an ordinary error body is read in full, so the connection goes back to the pool; past
// that, paying for a new connection is the cheaper side.
const discardedBodyLimit = 4 << 10

// drainAndCloseResponse releases a response the caller will never see.
//
// Draining before closing is what returns the connection to the pool: net/http only reuses a
// connection whose body reached EOF, so a Close on an unread body makes it abandon the connection
// instead.
func drainAndCloseResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	_, _ = io.CopyN(io.Discard, resp.Body, discardedBodyLimit)
	_ = resp.Body.Close()
}
