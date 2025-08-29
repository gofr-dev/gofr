package dbresolver

import (
	"sync/atomic"
	"time"
)

// Circuit breaker for replica health with atomic operations.
type circuitBreaker struct {
	failures    atomic.Int32
	lastFailure atomic.Int64
	state       atomic.Int32 // 0=closed, 1=open, 2=half-open.
	maxFailures int32
	timeout     time.Duration
}

func newCircuitBreaker(maxFailures int32, timeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
	}
}

func (cb *circuitBreaker) allowRequest() bool {
	state := cb.state.Load()

	switch state {
	case circuitStateClosed:
		return true
	case circuitStateOpen:
		if time.Since(time.Unix(0, cb.lastFailure.Load())) > cb.timeout {
			return cb.state.CompareAndSwap(circuitStateOpen, circuitStateHalfOpen)
		}

		return false
	case circuitStateHalfOpen:
		return true
	default:
		return true
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures.Store(0)
	cb.state.Store(0)
}

func (cb *circuitBreaker) recordFailure() {
	failures := cb.failures.Add(1)
	cb.lastFailure.Store(time.Now().UnixNano())

	if failures >= cb.maxFailures {
		cb.state.Store(1)
	}
}
