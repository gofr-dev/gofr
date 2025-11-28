package dbresolver

import (
	"sync/atomic"
	"time"
)

type circuitBreakerState int32

const (
	circuitStateClosed   circuitBreakerState = 0
	circuitStateOpen     circuitBreakerState = 1
	circuitStateHalfOpen circuitBreakerState = 2
)

// Circuit breaker for replica health with atomic operations.
type circuitBreaker struct {
	failures    atomic.Int32
	lastFailure atomic.Pointer[time.Time]
	state       atomic.Pointer[circuitBreakerState]
	maxFailures int32
	timeout     time.Duration
}

func newCircuitBreaker(maxFailures int32, timeout time.Duration) *circuitBreaker {
	cb := &circuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
	}

	// Initialize state to closed
	initialState := circuitStateClosed
	cb.state.Store(&initialState)

	return cb
}

func (cb *circuitBreaker) allowRequest() bool {
	state := cb.state.Load()
	if *state != circuitStateOpen {
		return true
	}

	lastFailurePtr := cb.lastFailure.Load()
	if lastFailurePtr == nil {
		return true
	}

	if time.Since(*lastFailurePtr) <= cb.timeout {
		return false
	}

	// Try to transition from open to half-open
	openState := circuitStateOpen
	halfOpenState := circuitStateHalfOpen

	return cb.state.CompareAndSwap(&openState, &halfOpenState)
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures.Store(0)

	// Reset lastFailure to nil to indicate no recent failures
	cb.lastFailure.Store(nil)

	closedState := circuitStateClosed

	cb.state.Store(&closedState)
}

func (cb *circuitBreaker) recordFailure() {
	failures := cb.failures.Add(1)
	now := time.Now()
	cb.lastFailure.Store(&now)

	if failures >= cb.maxFailures {
		openState := circuitStateOpen

		cb.state.Store(&openState)
	}
}
