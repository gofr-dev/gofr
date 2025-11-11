package dbresolver

import "time"

// Option is a function type for configuring the resolver.
type Option func(*Resolver)

// WithStrategy sets the strategy for the resolver.
func WithStrategy(strategy Strategy) Option {
	return func(r *Resolver) {
		r.strategy = strategy
	}
}

// WithFallback sets whether to fallback to primary on replica failure.
func WithFallback(fallback bool) Option {
	return func(r *Resolver) {
		r.readFallback = fallback
	}
}

func WithPrimaryRoutes(routes map[string]bool) Option {
	return func(r *Resolver) {
		r.primaryRoutes = routes
	}
}

func WithCircuitBreaker(maxFailures int32, timeoutSec int) Option {
	return func(r *Resolver) {
		timeout := time.Duration(timeoutSec) * time.Second

		for _, wrapper := range r.replicas {
			wrapper.breaker = newCircuitBreaker(maxFailures, timeout)
		}
	}
}
