package dbresolver

import (
	"context"
	"database/sql"
	"go.opentelemetry.io/otel/trace"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

var (
	readQueryPrefixRegex = regexp.MustCompile(`(?i)^\s*(SELECT|SHOW|DESCRIBE|EXPLAIN)`)
)

// Config holds configuration for DB resolver
type Config struct {
	ReplicaHosts   []string
	StrategyName   string
	FallbackToMain bool
	ReplicaUser    string
	ReplicaPass    string
	ReplicaPort    string
}

// Resolver implements the DB interface and routes queries to primary or replicas
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
	primaryReads  atomic.Uint64
	primaryWrites atomic.Uint64
	replicaReads  atomic.Uint64
}

// New creates a new resolver with the given primary and replicas
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
	if r.metrics != nil {
		r.metrics.NewHistogram("app_dbresolve_stats",
			"Response time of SQL operations in milliseconds",
			.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10)
	}

	r.logger.Logf("DB Resolver initialized with %d replicas using %s strategy",
		len(replicas), r.strategy.Name())

	// Start health monitoring
	go r.monitorHealth(context.Background())

	return r
}

// IsReadQuery determines if a query is a read operation
func IsReadQuery(query string) bool {
	return readQueryPrefixRegex.MatchString(query)
}

// Query routes to a replica if it's a read query, otherwise to primary
func (r *Resolver) Query(query string, args ...any) (*sql.Rows, error) {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		defer r.recordQueryStats("Query", "read", "replica", query, startTime)

		rows, err := db.Query(query, args...)
		if err != nil && r.readFallback {
			r.logger.Debugf("Failed to execute query on replica, falling back to primary: %v", err)
			r.stats.primaryReads.Add(1)
			defer r.recordQueryStats("Query", "read", "primary_fallback", query, startTime)
			return r.primary.Query(query, args...)
		}

		r.stats.replicaReads.Add(1)
		return rows, err
	}

	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("Query", "write", "primary", query, startTime)
	return r.primary.Query(query, args...)
}

// QueryContext routes to a replica if it's a read query, otherwise to primary
func (r *Resolver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		defer r.recordQueryStats("QueryContext", "read", "replica", query, startTime)

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil && r.readFallback {
			r.logger.Debugf("Failed to execute query on replica, falling back to primary: %v", err)
			r.stats.primaryReads.Add(1)
			defer r.recordQueryStats("QueryContext", "read", "primary_fallback", query, startTime)
			return r.primary.QueryContext(ctx, query, args...)
		}

		r.stats.replicaReads.Add(1)
		return rows, err
	}

	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("QueryContext", "write", "primary", query, startTime)
	return r.primary.QueryContext(ctx, query, args...)
}

// QueryRow routes to a replica if it's a read query, otherwise to primary
func (r *Resolver) QueryRow(query string, args ...any) *sql.Row {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		r.stats.replicaReads.Add(1)
		defer r.recordQueryStats("QueryRow", "read", "replica", query, startTime)
		return db.QueryRow(query, args...)
	}

	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("QueryRow", "write", "primary", query, startTime)
	return r.primary.QueryRow(query, args...)
}

// QueryRowContext routes to a replica if it's a read query, otherwise to primary
func (r *Resolver) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		r.stats.replicaReads.Add(1)
		defer r.recordQueryStats("QueryRowContext", "read", "replica", query, startTime)
		return db.QueryRowContext(ctx, query, args...)
	}

	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("QueryRowContext", "write", "primary", query, startTime)
	return r.primary.QueryRowContext(ctx, query, args...)
}

// Exec always routes to primary as it's a write operation
func (r *Resolver) Exec(query string, args ...any) (sql.Result, error) {
	startTime := time.Now()
	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("Exec", "write", "primary", query, startTime)
	return r.primary.Exec(query, args...)
}

// ExecContext always routes to primary as it's a write operation
func (r *Resolver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	startTime := time.Now()
	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("ExecContext", "write", "primary", query, startTime)
	return r.primary.ExecContext(ctx, query, args...)
}

// Prepare always uses primary as prepared statements are usually for writes
func (r *Resolver) Prepare(query string) (*sql.Stmt, error) {
	startTime := time.Now()
	defer r.recordQueryStats("Prepare", "unknown", "primary", query, startTime)
	return r.primary.Prepare(query)
}

// Begin returns a transaction which always uses the primary
func (r *Resolver) Begin() (*gofrSQL.Tx, error) {
	startTime := time.Now()
	defer r.recordQueryStats("Begin", "write", "primary", "", startTime)
	return r.primary.Begin()
}

// Select routes to a replica if it's a read query, otherwise to primary
func (r *Resolver) Select(ctx context.Context, data any, query string, args ...any) {
	startTime := time.Now()
	isRead := IsReadQuery(query)

	if isRead && len(r.replicas) > 0 {
		db := r.strategy.Choose(r.replicas)
		r.stats.replicaReads.Add(1)
		defer r.recordQueryStats("Select", "read", "replica", query, startTime)
		db.Select(ctx, data, query, args...)
		return
	}

	r.stats.primaryWrites.Add(1)
	defer r.recordQueryStats("Select", "write", "primary", query, startTime)
	r.primary.Select(ctx, data, query, args...)
}

// HealthCheck returns the health of both primary and replicas
func (r *Resolver) HealthCheck() *datasource.Health {
	primaryHealth := r.primary.HealthCheck()

	health := &datasource.Health{
		Status: primaryHealth.Status,
		Details: map[string]interface{}{
			"primary":  primaryHealth,
			"replicas": make([]interface{}, 0, len(r.replicas)),
			"stats": map[string]interface{}{
				"primaryReads":  r.stats.primaryReads.Load(),
				"primaryWrites": r.stats.primaryWrites.Load(),
				"replicaReads":  r.stats.replicaReads.Load(),
			},
		},
	}

	replicaDetails := make([]interface{}, 0, len(r.replicas))
	for i, replica := range r.replicas {
		replicaHealth := replica.HealthCheck()
		replicaDetails = append(replicaDetails, map[string]interface{}{
			"index":  i,
			"health": replicaHealth,
		})
	}
	health.Details["replicas"] = replicaDetails

	return health
}

// Dialect returns the primary dialect
func (r *Resolver) Dialect() string {
	return r.primary.Dialect()
}

// Close closes all connections
func (r *Resolver) Close() error {
	r.logger.Logf("Closing DB Resolver connections (1 primary, %d replicas)", len(r.replicas))

	// Close primary
	primaryErr := r.primary.Close()

	// Close all replicas
	for i, replica := range r.replicas {
		if err := replica.Close(); err != nil {
			r.logger.Errorf("Failed to close replica %d: %v", i, err)
			if primaryErr == nil {
				primaryErr = err
			}
		}
	}

	return primaryErr
}

// Primary returns the primary DB for direct access
func (r *Resolver) Primary() container.DB {
	return r.primary
}

// Replica returns a replica DB for direct access
func (r *Resolver) Replica() container.DB {
	if len(r.replicas) == 0 {
		return r.primary
	}
	return r.strategy.Choose(r.replicas)
}

// monitorHealth periodically checks the health of replicas
func (r *Resolver) monitorHealth(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkReplicasHealth()
		}
	}
}

// checkReplicasHealth checks the health of all replicas
func (r *Resolver) checkReplicasHealth() {
	for i, replica := range r.replicas {
		health := replica.HealthCheck()
		if health.Status != "UP" {
			r.logger.Warnf("Replica %d is not healthy: %s", i, health.Status)
		}
	}
}

// recordQueryStats records metrics and logs query execution
func (r *Resolver) recordQueryStats(operation, queryType, target, query string, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()

	// Log the query
	r.logger.Debug(&QueryLog{
		Query:     query,
		Duration:  duration,
		Operation: operation,
		Target:    target,
		QueryType: queryType,
	})

	// Record metrics
	if r.metrics != nil {
		r.metrics.RecordHistogram(context.Background(), "app_dbresolve_stats", float64(duration),
			"operation", operation,
			"type", queryType,
			"target", target,
		)
	}
}

// Option is a function that configures a Resolver
type Option func(*Resolver)

// WithStrategy sets the replica selection strategy
func WithStrategy(strategyName string) Option {
	return func(r *Resolver) {
		switch strings.ToLower(strategyName) {
		case "random":
			r.strategy = NewRandomStrategy()
		default:
			r.strategy = NewRoundRobinStrategy(len(r.replicas))
		}
	}
}

// WithRoundRobin sets the round-robin strategy
func WithRoundRobin() Option {
	return func(r *Resolver) {
		r.strategy = NewRoundRobinStrategy(len(r.replicas))
	}
}

// WithRandom sets the random strategy
func WithRandom() Option {
	return func(r *Resolver) {
		r.strategy = NewRandomStrategy()
	}
}

// WithReadFallback enables/disables fallback to primary for failed reads
func WithReadFallback(enabled bool) Option {
	return func(r *Resolver) {
		r.readFallback = enabled
	}
}
