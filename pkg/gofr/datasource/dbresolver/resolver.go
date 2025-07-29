package dbresolver

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

var (
	readQueryPrefixRegex = regexp.MustCompile(`(?i)^\s*(SELECT|SHOW|DESCRIBE|EXPLAIN)`)
)

const (
	healthStatusUP      = "UP"
	healthStatusDOWN    = "DOWN"
	roundRobinStrategy  = "round-robin"
	randomStrategy      = "random"
	metricsPushInterval = 10 * time.Second
	cleanupInterval     = 10 * time.Second
)

// Config holds configuration for DB resolver.
type Config struct {
	ReplicaHosts   []string
	StrategyName   string
	FallbackToMain bool
	ReplicaUser    string
	ReplicaPass    string
	ReplicaPort    string
}

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

// WithTracer sets the tracer for the resolver.
func WithTracer(tracer trace.Tracer) Option {
	return func(r *Resolver) {
		r.tracer = tracer
	}
}

// Resolver implements the DB interface and routes queries to primary or replicas.
type Resolver struct {
	primary      container.DB
	replicas     []container.DB
	strategy     Strategy
	readFallback bool
	logger       Logger
	metrics      Metrics
	tracer       trace.Tracer
	stats        *statistics
}

type statistics struct {
	primaryReads     atomic.Uint64
	primaryWrites    atomic.Uint64
	replicaReads     atomic.Uint64
	primaryFallbacks atomic.Uint64
	replicaFailures  atomic.Uint64
	totalQueries     atomic.Uint64
}

// New creates a new resolver with the given primary and replicas.
func New(primary container.DB, replicas []container.DB, logger Logger,
	metrics Metrics, opts ...Option) container.DB {
	if primary == nil {
		panic("primary database cannot be nil")
	}

	r := &Resolver{
		primary:      primary,
		replicas:     replicas,
		readFallback: true,
		logger:       logger,
		metrics:      metrics,
		stats:        &statistics{},
	}

	// Default to round-robin strategy
	r.strategy = NewRoundRobinStrategy(len(replicas))

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	// Initialize metrics
	r.initializeMetrics()

	r.logger.Logf("DB Resolver initialized with %d replicas using %s strategy",
		len(replicas), r.strategy.Name())

	// Start health monitoring
	go r.monitorHealth(context.Background())

	return r
}

// initializeMetrics sets up all DB resolver metrics following GoFr patterns.

func (r *Resolver) initializeMetrics() {
	if r.metrics == nil {
		return
	}

	// Histogram for query response times (following GoFr SQL pattern).
	dbResolverBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	r.metrics.NewHistogram("app_dbresolve_stats",
		"Response time of DB resolver operations in microseconds", dbResolverBuckets...)

	// Gauges for operation tracking
	r.metrics.NewGauge("app_dbresolve_primary_reads", "Total number of reads routed to primary")
	r.metrics.NewGauge("app_dbresolve_primary_writes", "Total number of writes routed to primary")
	r.metrics.NewGauge("app_dbresolve_replica_reads", "Total number of reads routed to replicas")
	r.metrics.NewGauge("app_dbresolve_fallbacks", "Total number of replica fallbacks to primary")
	r.metrics.NewGauge("app_dbresolve_replica_failures", "Total number of replica failures")
	r.metrics.NewGauge("app_dbresolve_total_queries", "Total number of queries processed")

	// Start metrics collection goroutine.
	go r.pushMetrics()
}

// pushMetrics continuously updates gauge metrics.
func (r *Resolver) pushMetrics() {
	ticker := time.NewTicker(metricsPushInterval)
	defer ticker.Stop()

	for range ticker.C {
		if r.metrics != nil {
			r.metrics.SetGauge("app_dbresolve_primary_reads", float64(r.stats.primaryReads.Load()))
			r.metrics.SetGauge("app_dbresolve_primary_writes", float64(r.stats.primaryWrites.Load()))
			r.metrics.SetGauge("app_dbresolve_replica_reads", float64(r.stats.replicaReads.Load()))
			r.metrics.SetGauge("app_dbresolve_fallbacks", float64(r.stats.primaryFallbacks.Load()))
			r.metrics.SetGauge("app_dbresolve_replica_failures", float64(r.stats.replicaFailures.Load()))
			r.metrics.SetGauge("app_dbresolve_total_queries", float64(r.stats.totalQueries.Load()))
		}
	}
}

// addTrace starts a new trace span following GoFr patterns.
func (r *Resolver) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
	if r.tracer != nil {
		tracedCtx, span := r.tracer.Start(ctx, fmt.Sprintf("dbresolve-%s", method))

		span.SetAttributes(
			attribute.String("dbresolve.query", query),
			attribute.String("dbresolve.method", method),
		)

		return tracedCtx, span
	}

	return ctx, nil
}

// IsReadQuery determines if a query is a read operation.
func IsReadQuery(query string) bool {
	return readQueryPrefixRegex.MatchString(query)
}

// Dialect returns the database dialect (required by container.DB interface).
func (r *Resolver) Dialect() string {
	return r.primary.Dialect()
}

// Close closes all database connections (required by container.DB interface).
func (r *Resolver) Close() error {
	var lastErr error

	// Close primary
	if err := r.primary.Close(); err != nil {
		lastErr = err
	}

	// Close all replicas
	for _, replica := range r.replicas {
		if err := replica.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Query routes to a replica if it's a read query, otherwise to primary.
func (r *Resolver) Query(query string, args ...any) (*sql.Rows, error) {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	r.stats.totalQueries.Add(1)

	ctx := context.Background()

	tracedCtx, span := r.addTrace(ctx, "query", query)
	defer r.sendOperationStats(startTime, "Query", query, "query", span, isRead, args...)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)

		rows, err := db.Query(query, args...)
		if err != nil && r.readFallback {
			r.logger.Debugf("Failed to execute query on replica, falling back to primary: %v", err)
			r.stats.replicaFailures.Add(1)
			r.stats.primaryFallbacks.Add(1)
			r.stats.primaryReads.Add(1)

			if span != nil {
				span.SetAttributes(attribute.Bool("dbresolve.fallback", true))
			}

			return r.primary.QueryContext(tracedCtx, query, args...)
		}

		r.stats.replicaReads.Add(1)

		if span != nil {
			span.SetAttributes(attribute.String("dbresolve.target", "replica"))
		}

		return rows, err
	}

	r.stats.primaryWrites.Add(1)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.QueryContext(tracedCtx, query, args...)
}

// QueryContext routes to a replica if it's a read query, otherwise to primary.
func (r *Resolver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "query-context", query)
	defer r.sendOperationStats(startTime, "QueryContext", query, "query", span, isRead, args...)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)

		rows, err := db.QueryContext(tracedCtx, query, args...)
		if err != nil && r.readFallback {
			r.logger.Debugf("Failed to execute query on replica, falling back to primary: %v", err)
			r.stats.replicaFailures.Add(1)
			r.stats.primaryFallbacks.Add(1)
			r.stats.primaryReads.Add(1)

			if span != nil {
				span.SetAttributes(attribute.Bool("dbresolve.fallback", true))
			}

			return r.primary.QueryContext(tracedCtx, query, args...)
		}

		r.stats.replicaReads.Add(1)

		if span != nil {
			span.SetAttributes(attribute.String("dbresolve.target", "replica"))
		}

		return rows, err
	}

	r.stats.primaryWrites.Add(1)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.QueryContext(tracedCtx, query, args...)
}

// QueryRow routes to a replica if it's a read query, otherwise to primary.
func (r *Resolver) QueryRow(query string, args ...any) *sql.Row {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	r.stats.totalQueries.Add(1)

	ctx := context.Background()

	tracedCtx, span := r.addTrace(ctx, "query-row", query)
	defer r.sendOperationStats(startTime, "QueryRow", query, "query", span, isRead, args...)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		r.stats.replicaReads.Add(1)

		if span != nil {
			span.SetAttributes(attribute.String("dbresolve.target", "replica"))
		}

		return db.QueryRowContext(tracedCtx, query, args...)
	}

	r.stats.primaryWrites.Add(1)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.QueryRowContext(tracedCtx, query, args...)
}

// QueryRowContext routes to a replica if it's a read query, otherwise to primary.
func (r *Resolver) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "query-row-context", query)
	defer r.sendOperationStats(startTime, "QueryRowContext", query, "query", span, isRead, args...)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		r.stats.replicaReads.Add(1)

		if span != nil {
			span.SetAttributes(attribute.String("dbresolve.target", "replica"))
		}

		return db.QueryRowContext(tracedCtx, query, args...)
	}

	r.stats.primaryWrites.Add(1)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.QueryRowContext(tracedCtx, query, args...)
}

// ExecContext always routes to primary as it's a write operation.
func (r *Resolver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	startTime := time.Now()

	r.stats.primaryWrites.Add(1)
	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "exec-context", query)
	defer r.sendOperationStats(startTime, "ExecContext", query, "exec", span, false, args...)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.ExecContext(tracedCtx, query, args...)
}

// Exec always routes to primary as it's a write operation.
func (r *Resolver) Exec(query string, args ...any) (sql.Result, error) {
	startTime := time.Now()

	r.stats.primaryWrites.Add(1)
	r.stats.totalQueries.Add(1)

	ctx := context.Background()

	tracedCtx, span := r.addTrace(ctx, "exec", query)
	defer r.sendOperationStats(startTime, "Exec", query, "exec", span, false, args...)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.ExecContext(tracedCtx, query, args...)
}

// Select routes to a replica if it's a read query, otherwise to primary.
func (r *Resolver) Select(ctx context.Context, data any, query string, args ...any) {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	r.stats.totalQueries.Add(1)

	tracedCtx, span := r.addTrace(ctx, "select", query)
	defer r.sendOperationStats(startTime, "Select", query, "select", span, isRead, args...)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		r.stats.replicaReads.Add(1)

		if span != nil {
			span.SetAttributes(attribute.String("dbresolve.target", "replica"))
		}

		db.Select(tracedCtx, data, query, args...)

		return
	}

	r.stats.primaryWrites.Add(1)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	r.primary.Select(tracedCtx, data, query, args...)
}

// Prepare routes to primary (prepared statements should be consistent).
func (r *Resolver) Prepare(query string) (*sql.Stmt, error) {
	startTime := time.Now()

	r.stats.totalQueries.Add(1)

	ctx := context.Background()

	_, span := r.addTrace(ctx, "prepare", query)
	defer r.sendOperationStats(startTime, "Prepare", query, "prepare", span, false)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.Prepare(query)
}

// PrepareContext routes to primary (prepared statements should be consistent).
func (r *Resolver) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	startTime := time.Now()

	r.stats.totalQueries.Add(1)

	_, span := r.addTrace(ctx, "prepare-context", query)
	defer r.sendOperationStats(startTime, "PrepareContext", query, "prepare", span, false)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.Prepare(query)
}

// Begin always routes to primary as transactions should be on primary.
func (r *Resolver) Begin() (*gofrSQL.Tx, error) {
	startTime := time.Now()

	r.stats.totalQueries.Add(1)

	ctx := context.Background()

	_, span := r.addTrace(ctx, "begin", "BEGIN")
	defer r.sendOperationStats(startTime, "Begin", "BEGIN", "transaction", span, false)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.Begin()
}

// BeginTx always routes to primary as transactions should be on primary.
func (r *Resolver) BeginTx(ctx context.Context) (*gofrSQL.Tx, error) {
	startTime := time.Now()

	r.stats.totalQueries.Add(1)

	_, span := r.addTrace(ctx, "begin-tx", "BEGIN")
	defer r.sendOperationStats(startTime, "BeginTx", "BEGIN", "transaction", span, false)

	if span != nil {
		span.SetAttributes(attribute.String("dbresolve.target", "primary"))
	}

	return r.primary.Begin()
}

// HealthCheck returns comprehensive health information.
func (r *Resolver) HealthCheck() *datasource.Health {
	primaryHealth := r.primary.HealthCheck()

	health := &datasource.Health{
		Status: primaryHealth.Status,
		Details: map[string]any{
			"primary":  primaryHealth,
			"replicas": make([]any, 0, len(r.replicas)),
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

	replicaDetails := make([]any, 0, len(r.replicas))

	for i, replica := range r.replicas {
		replicaHealth := replica.HealthCheck()

		replicaDetails = append(replicaDetails, map[string]any{
			"index":  i,
			"health": replicaHealth,
		})
	}

	health.Details["replicas"] = replicaDetails

	return health
}

// sendOperationStats records metrics and logs following GoFr patterns.
func (r *Resolver) sendOperationStats(startTime time.Time, methodType, query string,
	method string, span trace.Span, isRead bool, args ...any) {
	duration := time.Since(startTime).Microseconds()

	// Log following GoFr pattern.
	r.logger.Debug(&Log{
		Type:     methodType,
		Query:    query,
		Duration: duration,
		IsRead:   isRead,
		Args:     args,
	})

	// Set trace attributes.
	if span != nil {
		defer span.End()
		span.SetAttributes(
			attribute.Int64(fmt.Sprintf("dbresolve.%s.duration", method), duration),
			attribute.Bool("dbresolve.is_read", isRead),
		)
	}

	// Record histogram metrics.
	if r.metrics != nil {
		target := "primary"
		if isRead && len(r.replicas) > 0 {
			target = "replica"
		}

		r.metrics.RecordHistogram(context.Background(), "app_dbresolve_stats",
			float64(duration),
			"operation", methodType,
			"target", target,
			"type", getOperationType(query))
	}
}

// getOperationType extracts operation type from query.
func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Split(query, " ")

	if len(words) > 0 {
		return strings.ToUpper(words[0])
	}

	return "UNKNOWN"
}

// Log structure following GoFr patterns.
type Log struct {
	Type     string `json:"type"`
	Query    string `json:"query"`
	Duration int64  `json:"duration"`
	IsRead   bool   `json:"is_read"`
	Args     []any  `json:"args,omitempty"`
}

// monitorHealth continuously monitors the health of all connections.
func (r *Resolver) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check primary health.
			primaryHealth := r.primary.HealthCheck()
			if primaryHealth.Status != healthStatusUP {
				r.logger.Logf("Primary database health check failed: %v", primaryHealth)
			}

			// Check replica health.
			for i, replica := range r.replicas {
				replicaHealth := replica.HealthCheck()
				if replicaHealth.Status != healthStatusUP {
					r.logger.Logf("Replica %d health check failed: %v", i, replicaHealth)
				}
			}
		}
	}
}
