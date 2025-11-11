package dbresolver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

// Constants for strategies and intervals.
const (
	contextKeyHTTPMethod  contextKey = "dbresolver.http_method"
	contextKeyRequestPath contextKey = "dbresolver.request_path"

	defaultMaxFailures = 5
	defaultTimeoutSec  = 30
)

var errReplicaFailedNoFallback = errors.New("replica query failed and fallback disabled")

type contextKey string

// statistics holds atomic counters for various operations.
type statistics struct {
	primaryReads     atomic.Uint64
	primaryWrites    atomic.Uint64
	replicaReads     atomic.Uint64
	primaryFallbacks atomic.Uint64
	replicaFailures  atomic.Uint64
	totalQueries     atomic.Uint64
}

// Replica wrapper with circuit breaker.
type replicaWrapper struct {
	db      container.DB
	breaker *circuitBreaker
	index   int
}

// Resolver is the main struct that implements the container.DB interface.
type Resolver struct {
	primary      container.DB
	replicas     []*replicaWrapper
	strategy     Strategy
	readFallback bool

	logger  Logger
	metrics Metrics
	tracer  trace.Tracer

	primaryRoutes   map[string]bool
	primaryPrefixes []string

	stats *statistics

	// Background task management.
	stopChan chan struct{}
	wg       sync.WaitGroup
	once     sync.Once
}

// NewResolver creates a new Resolver instance with the provided primary and replicas.
func NewResolver(primary container.DB, replicas []container.DB, logger Logger, metrics Metrics, opts ...Option) container.DB {
	// Wrap replicas with circuit breakers
	replicaWrappers := make([]*replicaWrapper, len(replicas))
	for i, replica := range replicas {
		replicaWrappers[i] = &replicaWrapper{
			db:      replica,
			breaker: newCircuitBreaker(defaultMaxFailures, time.Duration(defaultTimeoutSec)*time.Second),
			index:   i,
		}
	}

	r := &Resolver{
		primary:       primary,
		replicas:      replicaWrappers,
		readFallback:  true, // Default to true
		logger:        logger,
		metrics:       metrics,
		stats:         &statistics{},
		primaryRoutes: make(map[string]bool),
		stopChan:      make(chan struct{}),
	}

	// Default strategy
	if len(replicas) > 0 {
		r.strategy = NewRoundRobinStrategy()
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	for route := range r.primaryRoutes {
		if strings.HasSuffix(route, "*") {
			r.primaryPrefixes = append(r.primaryPrefixes, strings.TrimSuffix(route, "*"))
		}
	}

	// Initialize metrics and start background tasks.
	r.initializeMetrics()
	r.startBackgroundTasks()

	if r.logger != nil {
		r.logger.Logf("DB Resolver initialized with %d replicas using circuit breakers", len(replicas))
	}

	return r
}

// initializeMetrics sets up metrics following GoFr patterns.
func (r *Resolver) initializeMetrics() {
	if r.metrics == nil {
		return
	}

	// Histogram for query response times
	buckets := []float64{0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0}
	r.metrics.NewHistogram("dbresolver_query_duration", "Response time of DB resolver operations in microseconds", buckets...)

	// Gauges for operation tracking
	r.metrics.NewGauge("dbresolver_primary_reads", "Total reads routed to primary")
	r.metrics.NewGauge("dbresolver_primary_writes", "Total writes routed to primary")
	r.metrics.NewGauge("dbresolver_replica_reads", "Total reads routed to replicas")
	r.metrics.NewGauge("dbresolver_fallbacks", "Total fallbacks to primary")
	r.metrics.NewGauge("dbresolver_failures", "Total replica failures")
}

// startBackgroundTasks starts minimal background processing.
func (r *Resolver) startBackgroundTasks() {
	r.wg.Add(1)

	go r.backgroundProcessor()
}

// backgroundProcessor handles metrics collection with reduced frequency.
func (r *Resolver) backgroundProcessor() {
	defer r.wg.Done()

	ticker := time.NewTicker(time.Duration(defaultTimeoutSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.updateMetrics()
		}
	}
}

// updateMetrics updates gauge metrics.
func (r *Resolver) updateMetrics() {
	if r.metrics == nil {
		return
	}

	r.metrics.SetGauge("dbresolver_primary_reads", float64(r.stats.primaryReads.Load()))
	r.metrics.SetGauge("dbresolver_primary_writes", float64(r.stats.primaryWrites.Load()))
	r.metrics.SetGauge("dbresolver_replica_reads", float64(r.stats.replicaReads.Load()))
	r.metrics.SetGauge("dbresolver_fallbacks", float64(r.stats.primaryFallbacks.Load()))
	r.metrics.SetGauge("dbresolver_failures", float64(r.stats.replicaFailures.Load()))
}

// shouldUseReplica determines routing based on HTTP method and path.
func (r *Resolver) shouldUseReplica(ctx context.Context) bool {
	if len(r.replicas) == 0 {
		return false
	}

	// Check if path requires primary.
	if path, ok := ctx.Value(contextKeyRequestPath).(string); ok {
		if r.isPrimaryRoute(path) {
			return false
		}
	}

	// Check HTTP method.
	method, ok := ctx.Value(contextKeyHTTPMethod).(string)
	if !ok {
		return false // Default to primary for safety.
	}

	return method == "GET" || method == "HEAD" || method == "OPTIONS"
}

// isPrimaryRoute checks if path matches primary route patterns.
func (r *Resolver) isPrimaryRoute(path string) bool {
	if r.primaryRoutes[path] {
		return true
	}

	// Prefix match (precompiled patterns)
	for _, prefix := range r.primaryPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// selectHealthyReplica chooses an available replica using circuit breaker.
func (r *Resolver) selectHealthyReplica() *replicaWrapper {
	if len(r.replicas) == 0 {
		return nil
	}

	// Get all available DBs for strategy.
	var (
		availableDbs      []container.DB
		availableWrappers []*replicaWrapper
	)

	for _, wrapper := range r.replicas {
		if wrapper.breaker.allowRequest() {
			availableDbs = append(availableDbs, wrapper.db)
			availableWrappers = append(availableWrappers, wrapper)
		}
	}

	if len(availableDbs) == 0 {
		if r.logger != nil {
			r.logger.Warn("All replicas are unavailable (circuit breakers open), falling back to primary")
		}

		return nil
	}

	// Use strategy to choose from available replicas.
	chosenDB, err := r.strategy.Choose(availableDbs)
	if err != nil {
		return nil
	}

	for _, wrapper := range availableWrappers {
		if wrapper.db == chosenDB {
			return wrapper
		}
	}

	return availableWrappers[0]
}

// addTrace adds tracing information to the context and returns a span.
func (r *Resolver) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
	if r.tracer == nil {
		return ctx, nil
	}

	tracedCtx, span := r.tracer.Start(ctx, fmt.Sprintf("dbresolver-%s", method))
	if span != nil {
		span.SetAttributes(
			attribute.String("dbresolver.query", query),
			attribute.String("dbresolver.method", method),
		)
	}

	return tracedCtx, span
}

// recordStats records operation statistics and updates tracing spans.
func (r *Resolver) recordStats(start time.Time, method, target string, span trace.Span, isRead bool) {
	duration := time.Since(start).Microseconds()

	// Update trace if available.
	if span != nil {
		defer span.End()

		span.SetAttributes(
			attribute.String("dbresolver.target", target),
			attribute.Int64("dbresolver.duration", duration),
			attribute.Bool("dbresolver.is_read", isRead),
		)
	}

	// Record metrics histogram only if metrics are enabled.
	if r.metrics != nil {
		r.metrics.RecordHistogram(context.Background(), "dbresolver_query_duration",
			float64(duration), "method", method, "target", target)
	}
}

// Query routes to replica for reads, primary for writes.
func (r *Resolver) Query(query string, args ...any) (*sql.Rows, error) {
	return r.QueryContext(context.Background(), query, args...)
}

// QueryContext routes queries with optimized path.
func (r *Resolver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()

	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "query", query)
	useReplica := r.shouldUseReplica(ctx)

	if useReplica && len(r.replicas) > 0 {
		return r.executeReplicaQuery(tracedCtx, span, start, query, args...)
	}

	// Non-GET requests or no replicas - use primary.
	r.stats.primaryWrites.Add(1)
	rows, err := r.primary.QueryContext(tracedCtx, query, args...)

	r.recordStats(start, "query", "primary", span, false)

	return rows, err
}

// executeReplicaQuery attempts to execute query on replica with fallback handling.
func (r *Resolver) executeReplicaQuery(ctx context.Context, span trace.Span, start time.Time,
	query string, args ...any) (*sql.Rows, error) {
	wrapper := r.selectHealthyReplica()

	if wrapper == nil {
		return r.fallbackToPrimary(ctx, span, start, query, "No healthy replica available, falling back to primary", args...)
	}

	rows, err := wrapper.db.QueryContext(ctx, query, args...)
	if err == nil {
		r.stats.replicaReads.Add(1)
		wrapper.breaker.recordSuccess()

		r.recordStats(start, "query", "replica", span, true)

		return rows, nil
	}

	// Record failure.
	wrapper.breaker.recordFailure()
	r.stats.replicaFailures.Add(1)

	if r.logger != nil {
		r.logger.Errorf("Replica #%d failed, circuit breaker triggered: %v", wrapper.index+1, err)
	}

	return r.fallbackToPrimary(ctx, span, start, query, "Falling back to primary for read operation", args...)
}

// fallbackToPrimary handles primary fallback logic with custom warning message.
func (r *Resolver) fallbackToPrimary(ctx context.Context, span trace.Span, start time.Time,
	query, warningMsg string, args ...any) (*sql.Rows, error) {
	if !r.readFallback {
		r.recordStats(start, "query", "replica-failed", span, true)

		return nil, errReplicaFailedNoFallback
	}

	r.stats.primaryFallbacks.Add(1)
	r.stats.primaryReads.Add(1)

	if r.logger != nil && warningMsg != "" {
		r.logger.Warn(warningMsg)
	}

	rows, err := r.primary.QueryContext(ctx, query, args...)
	r.recordStats(start, "query", "primary-fallback", span, true)

	return rows, err
}

// QueryRow routes to replica for reads, primary for writes.
func (r *Resolver) QueryRow(query string, args ...any) *sql.Row {
	return r.QueryRowContext(context.Background(), query, args...)
}

// QueryRowContext routes queries with circuit breaker.
func (r *Resolver) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()

	r.stats.totalQueries.Add(1)

	useReplica := r.shouldUseReplica(ctx)

	tracedCtx, span := r.addTrace(ctx, "query-row", query)
	defer r.recordStats(start, "query-row", "primary", span, useReplica)

	if useReplica && len(r.replicas) > 0 {
		wrapper := r.selectHealthyReplica()
		if wrapper != nil {
			r.stats.replicaReads.Add(1)
			wrapper.breaker.recordSuccess()

			return wrapper.db.QueryRowContext(tracedCtx, query, args...)
		}

		r.stats.replicaFailures.Add(1)
	}

	r.stats.primaryWrites.Add(1)

	return r.primary.QueryRowContext(tracedCtx, query, args...)
}

// Exec always routes to primary (write operation).
func (r *Resolver) Exec(query string, args ...any) (sql.Result, error) {
	return r.ExecContext(context.Background(), query, args...)
}

// ExecContext always routes to primary (write operation).
func (r *Resolver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()

	r.stats.primaryWrites.Add(1)
	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "exec", query)
	defer r.recordStats(start, "exec", "primary", span, false)

	return r.primary.ExecContext(tracedCtx, query, args...)
}

// Select routes to replica for reads, primary for writes.
func (r *Resolver) Select(ctx context.Context, data any, query string, args ...any) {
	start := time.Now()

	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "select", query)

	useReplica := r.shouldUseReplica(ctx)

	if useReplica && len(r.replicas) > 0 {
		wrapper := r.selectHealthyReplica()

		if wrapper != nil {
			r.stats.replicaReads.Add(1)
			wrapper.breaker.recordSuccess()
			wrapper.db.Select(tracedCtx, data, query, args...)

			r.recordStats(start, "select", "replica", span, true)

			return
		}

		r.stats.replicaFailures.Add(1)
	}

	r.stats.primaryWrites.Add(1)

	r.primary.Select(tracedCtx, data, query, args...)

	r.recordStats(start, "select", "primary", span, false)
}

// Prepare always routes to primary (consistency).
func (r *Resolver) Prepare(query string) (*sql.Stmt, error) {
	r.stats.totalQueries.Add(1)

	return r.primary.Prepare(query)
}

// Begin always routes to primary (transactions).
func (r *Resolver) Begin() (*gofrSQL.Tx, error) {
	r.stats.totalQueries.Add(1)

	return r.primary.Begin()
}

// Dialect returns the database dialect.
func (r *Resolver) Dialect() string {
	return r.primary.Dialect()
}

// HealthCheck returns comprehensive health information.
func (r *Resolver) HealthCheck() *datasource.Health {
	primaryHealth := r.primary.HealthCheck()

	health := &datasource.Health{
		Status: primaryHealth.Status,
		Details: map[string]any{
			"primary": primaryHealth,
			"stats": map[string]any{
				"primaryReads":     r.stats.primaryReads.Load(),
				"primaryWrites":    r.stats.primaryWrites.Load(),
				"replicaReads":     r.stats.replicaReads.Load(),
				"primaryFallbacks": r.stats.primaryFallbacks.Load(),
				"replicaFailures":  r.stats.replicaFailures.Load(),
				"totalQueries":     r.stats.totalQueries.Load(),
			},
		},
	}

	// Check replica health with circuit breaker status
	replicaDetails := make([]any, len(r.replicas))

	for i, wrapper := range r.replicas {
		replicaHealth := wrapper.db.HealthCheck()
		state := wrapper.breaker.state.Load()

		var stateStr string

		switch *state {
		case circuitStateClosed:
			stateStr = "CLOSED"
		case circuitStateOpen:
			stateStr = "OPEN"
		case circuitStateHalfOpen:
			stateStr = "HALF_OPEN"
		}

		replicaDetails[i] = map[string]any{
			"index":         i,
			"health":        replicaHealth,
			"circuit_state": stateStr,
			"failures":      wrapper.breaker.failures.Load(),
		}
	}

	health.Details["replicas"] = replicaDetails

	return health
}

// Close cleans up resources properly.
func (r *Resolver) Close() error {
	var err error

	// Stop background tasks only once
	r.once.Do(func() {
		close(r.stopChan)
		r.wg.Wait()
	})

	// Close primary
	if closeErr := r.primary.Close(); closeErr != nil {
		err = closeErr
	}

	// Close replicas
	for _, wrapper := range r.replicas {
		if closeErr := wrapper.db.Close(); closeErr != nil {
			err = closeErr
		}
	}

	return err
}

// WithHTTPMethod adds HTTP method to context for routing decisions.
func WithHTTPMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, contextKeyHTTPMethod, method)
}

// WithRequestPath adds request path to context.
func WithRequestPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, contextKeyRequestPath, path)
}
