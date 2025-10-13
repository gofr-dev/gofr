package dbresolver

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
