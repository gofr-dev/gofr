package dbresolver

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
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
	healthStatusUP           = "UP"
	healthStatusDOWN         = "DOWN"
	defaultMaxFailures int32 = 5
	defaultTimeoutSec  int   = 30
	defaultCacheSize   int64 = 1000
	minQueryLength     int   = 4
)

// Pre-compiled regex - compiled once at package init.
var readQueryRegex = regexp.MustCompile(`(?i)^\s*(SELECT|SHOW|DESCRIBE|EXPLAIN)`)

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

	queryCache *queryCache
	stats      *statistics

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
		}
	}

	r := &Resolver{
		primary:      primary,
		replicas:     replicaWrappers,
		readFallback: true, // Default to true
		logger:       logger,
		metrics:      metrics,
		queryCache:   newQueryCache(defaultCacheSize), // Bounded sync.Map cache.
		stats:        &statistics{},
		stopChan:     make(chan struct{}),
	}

	// Default strategy
	if len(replicas) > 0 {
		r.strategy = NewRoundRobinStrategy()
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
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

// Fast query classification with optimized string operations.
func (r *Resolver) isReadQuery(query string) bool {
	// Fast path: check first few characters for common patterns
	if len(query) < minQueryLength {
		return false
	}

	// Trim whitespace and get first word
	trimmed := strings.TrimLeft(query, " \t\n\r")
	if len(trimmed) < minQueryLength {
		return false
	}

	// Fast string comparison for common cases
	firstFour := strings.ToUpper(trimmed[:minQueryLength])
	switch firstFour {
	case "SELE", "SHOW", "DESC", "EXPL":
		return true
	}

	// Check cache for edge cases
	if cached, exists := r.queryCache.get(query); exists {
		return cached
	}

	// Fallback to regex for complex queries
	isRead := readQueryRegex.MatchString(query)

	r.queryCache.set(query, isRead)

	return isRead
}

// selectHealthyReplica chooses an available replica using circuit breaker.
func (r *Resolver) selectHealthyReplica() (availableDB container.DB, availableIndex int) {
	if len(r.replicas) == 0 {
		return nil, -1
	}

	// Get all available DBs for strategy
	var (
		availableDbs     []container.DB
		availableIndexes []int
	)

	for i, wrapper := range r.replicas {
		if wrapper.breaker.allowRequest() {
			availableDbs = append(availableDbs, wrapper.db)
			availableIndexes = append(availableIndexes, i)
		}
	}

	if len(availableDbs) == 0 {
		return nil, -1
	}

	// Use strategy to choose from available replicas
	chosenDB, err := r.strategy.Choose(availableDbs)
	if err != nil {
		return nil, -1
	}

	// Find the index of chosen replica
	for i, db := range availableDbs {
		if db == chosenDB {
			return chosenDB, availableIndexes[i]
		}
	}

	return chosenDB, availableIndexes[0]
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
	isRead := r.isReadQuery(query)
	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "query", query)

	if isRead && len(r.replicas) > 0 {
		// Try replica first
		replica, replicaIdx := r.selectHealthyReplica()
		if replica != nil {
			rows, err := replica.QueryContext(tracedCtx, query, args...)
			if err == nil {
				r.stats.replicaReads.Add(1)
				r.replicas[replicaIdx].breaker.recordSuccess()
				r.recordStats(start, "query", "replica", span, true)

				return rows, nil
			}

			// Record failure
			r.replicas[replicaIdx].breaker.recordFailure()
			r.stats.replicaFailures.Add(1)
		}

		// Fallback to primary if enabled
		if r.readFallback {
			r.stats.primaryFallbacks.Add(1)
			r.stats.primaryReads.Add(1)
			rows, err := r.primary.QueryContext(tracedCtx, query, args...)
			r.recordStats(start, "query", "primary-fallback", span, true)

			return rows, err
		}

		r.recordStats(start, "query", "replica-failed", span, true)

		return nil, errReplicaFailedNoFallback
	}

	// Write query - always use primary
	r.stats.primaryWrites.Add(1)
	rows, err := r.primary.QueryContext(tracedCtx, query, args...)
	r.recordStats(start, "query", "primary", span, false)

	return rows, err
}

// QueryRow routes to replica for reads, primary for writes.
func (r *Resolver) QueryRow(query string, args ...any) *sql.Row {
	return r.QueryRowContext(context.Background(), query, args...)
}

// QueryRowContext routes queries with circuit breaker.
func (r *Resolver) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	isRead := r.isReadQuery(query)
	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "query-row", query)
	defer r.recordStats(start, "query-row", "primary", span, isRead)

	if isRead && len(r.replicas) > 0 {
		replica, replicaIdx := r.selectHealthyReplica()
		if replica != nil {
			r.stats.replicaReads.Add(1)
			r.replicas[replicaIdx].breaker.recordSuccess()

			return replica.QueryRowContext(tracedCtx, query, args...)
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
	isRead := r.isReadQuery(query)
	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "select", query)

	if isRead && len(r.replicas) > 0 {
		replica, replicaIdx := r.selectHealthyReplica()

		if replica != nil {
			r.stats.replicaReads.Add(1)
			r.replicas[replicaIdx].breaker.recordSuccess()
			replica.Select(tracedCtx, data, query, args...)
			r.recordStats(start, "select", "replica", span, true)

			return
		}

		r.stats.replicaFailures.Add(1)
	}

	r.stats.primaryWrites.Add(1)

	r.primary.Select(tracedCtx, data, query, args...)

	r.recordStats(start, "select", "primary", span, isRead)
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
