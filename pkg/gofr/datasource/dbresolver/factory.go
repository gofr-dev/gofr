package dbresolver

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

const (
	minIdleReplicaDefault    = 2
	maxIdleReplicaCapDefault = 10
	minOpenReplicaDefault    = 5
	maxOpenReplicaCapDefault = 20
)

var errPrimaryNil = errors.New("primary SQL connection is nil")

// Config holds resolver configuration
type Config struct {
	Strategy      StrategyType
	ReadFallback  bool
	MaxFailures   uint32
	TimeoutSec    uint32
	PrimaryRoutes []string
}

// DBResolverProvider implements container.DBResolverProvider interface
type DBResolverProvider struct {
	resolver container.DB
	logger   any
	metrics  any
	tracer   trace.Tracer
	cfg      Config
	app      *gofr.App
}

// NewDBResolverProvider creates a new DBResolverProvider
func NewDBResolverProvider(app *gofr.App, cfg Config) *DBResolverProvider {
	return &DBResolverProvider{
		app: app,
		cfg: cfg,
	}
}

// UseLogger sets the logger - ✅ Implements provider.UseLogger()
func (p *DBResolverProvider) UseLogger(logger any) {
	p.logger = logger
}

// UseMetrics sets the metrics - ✅ Implements provider.UseMetrics()
func (p *DBResolverProvider) UseMetrics(metrics any) {
	p.metrics = metrics
}

// UseTracer sets the tracer - ✅ Implements provider.UseTracer()
func (p *DBResolverProvider) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		p.tracer = t
	}
}

// Connect initializes the resolver - ✅ SINGLE SOURCE OF TRUTH
func (p *DBResolverProvider) Connect() {
	// Get primary from app
	primary := p.app.GetSQL()
	if primary == nil {
		if logger, ok := p.logger.(Logger); ok {
			logger.Errorf("Primary SQL connection is nil")
		}
		return
	}

	// Convert logger and metrics to proper types
	logger, loggerOk := p.logger.(Logger)
	metrics, metricsOk := p.metrics.(Metrics)

	if !loggerOk || !metricsOk {
		if loggerOk {
			logger.Errorf("Invalid logger or metrics type")
		}
		return
	}

	// Create replicas from config
	replicas, err := createReplicas(p.app.Config, logger, metrics)
	if err != nil {
		logger.Errorf("Failed to create replicas: %v", err)
		return
	}

	if len(replicas) == 0 {
		logger.Warn("No replicas configured - all operations will use primary")
		p.resolver = primary
		return
	}

	// Build primary routes map
	primaryRoutesMap := make(map[string]bool)
	for _, route := range p.cfg.PrimaryRoutes {
		primaryRoutesMap[route] = true
	}

	// Create strategy
	var strategy Strategy
	switch p.cfg.Strategy {
	case strategyRandom:
		strategy = NewRandomStrategy()
	case strategyRoundRobin:
		strategy = NewRoundRobinStrategy()
	default:
		strategy = NewRoundRobinStrategy()
	}

	// ✅ Create resolver - single place!
	p.resolver = NewResolver(
		primary,
		replicas,
		logger,
		metrics,
		WithStrategy(strategy),
		WithFallback(p.cfg.ReadFallback),
		WithPrimaryRoutes(primaryRoutesMap),
	)

	logger.Logf("DB Resolver initialized with %d replicas", len(replicas))
}

// GetResolver returns the underlying resolver - ✅ Implements DBResolverProvider.GetResolver()
func (p *DBResolverProvider) GetResolver() container.DB {
	return p.resolver
}

// InitDBResolver - Complete initialization with middleware
func InitDBResolver(app *gofr.App, cfg Config) error {
	provider := NewDBResolverProvider(app, cfg)

	// ✅ Add middleware to inject HTTP context
	app.UseMiddleware(createHTTPMiddleware())

	// ✅ Add resolver (calls Connect() internally)
	app.AddDBResolver(provider)

	return nil
}

// createHTTPMiddleware injects HTTP method/path into context
func createHTTPMiddleware() gofrHTTP.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = WithHTTPMethod(ctx, r.Method)
			ctx = WithRequestPath(ctx, r.URL.Path)

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// createReplicas creates replica connections from configuration
func createReplicas(cfg config.Config, logger Logger, metrics Metrics) ([]container.DB, error) {
	replicasStr := cfg.Get("DB_REPLICA_HOSTS")
	if replicasStr == "" {
		return nil, nil
	}

	replicaHosts := strings.Split(replicasStr, ",")
	replicas := make([]container.DB, 0, len(replicaHosts))

	user := cfg.GetOrDefault("DB_REPLICA_USER", cfg.Get("DB_USER"))
	password := cfg.GetOrDefault("DB_REPLICA_PASSWORD", cfg.Get("DB_PASSWORD"))

	for i, hostPort := range replicaHosts {
		hostPort = strings.TrimSpace(hostPort)
		if hostPort == "" {
			continue
		}

		parts := strings.Split(hostPort, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid replica host format at index %d: %s (expected host:port)", i, hostPort)
		}

		host, port := parts[0], parts[1]

		replica, err := createReplicaConnection(cfg, host, port, user, password, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create replica #%d (%s:%s): %w", i+1, host, port, err)
		}

		replicas = append(replicas, replica)
		logger.Logf("Created DB replica connection to %s:%s", host, port)
	}

	return replicas, nil
}

// createReplicaConnection creates a single replica database connection
func createReplicaConnection(cfg config.Config, host, port, user, password string, logger Logger, metrics Metrics) (container.DB, error) {
	dbName := cfg.Get("DB_NAME")
	if dbName == "" {
		return nil, errors.New("DB_NAME is required")
	}

	replicaCfg := &replicaConfig{
		base:     cfg,
		host:     host,
		port:     port,
		user:     user,
		password: password,
	}

	db := gofrSQL.NewSQL(replicaCfg, logger, metrics)
	return db, nil
}

// replicaConfig wraps the main config and overrides specific values
type replicaConfig struct {
	base     config.Config
	host     string
	port     string
	user     string
	password string
}

func (r *replicaConfig) Get(key string) string {
	switch key {
	case "DB_HOST":
		return r.host
	case "DB_PORT":
		return r.port
	case "DB_USER":
		return r.user
	case "DB_PASSWORD":
		return r.password
	case "DB_MAX_IDLE_CONNECTIONS":
		return optimizedIdleConnections(r.base)
	case "DB_MAX_OPEN_CONNECTIONS":
		return optimizedOpenConnections(r.base)
	default:
		return r.base.Get(key)
	}
}

func (r *replicaConfig) GetOrDefault(key, defaultValue string) string {
	val := r.Get(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func optimizedIdleConnections(cfg config.Config) string {
	maxIdleStr := cfg.Get("DB_REPLICA_MAX_IDLE_CONNECTIONS")
	if maxIdleStr == "" {
		return strconv.Itoa(minIdleReplicaDefault)
	}

	maxIdle, err := strconv.Atoi(maxIdleStr)
	if err != nil {
		return strconv.Itoa(minIdleReplicaDefault)
	}

	if maxIdle < minIdleReplicaDefault {
		maxIdle = minIdleReplicaDefault
	}
	if maxIdle > maxIdleReplicaCapDefault {
		maxIdle = maxIdleReplicaCapDefault
	}

	return strconv.Itoa(maxIdle)
}

func optimizedOpenConnections(cfg config.Config) string {
	maxOpenStr := cfg.Get("DB_REPLICA_MAX_OPEN_CONNECTIONS")
	if maxOpenStr == "" {
		return strconv.Itoa(minOpenReplicaDefault)
	}

	maxOpen, err := strconv.Atoi(maxOpenStr)
	if err != nil {
		return strconv.Itoa(minOpenReplicaDefault)
	}

	if maxOpen < minOpenReplicaDefault {
		maxOpen = minOpenReplicaDefault
	}
	if maxOpen > maxOpenReplicaCapDefault {
		maxOpen = maxOpenReplicaCapDefault
	}

	return strconv.Itoa(maxOpen)
}
