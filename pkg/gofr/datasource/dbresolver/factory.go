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
	expectedHostPortParts
)

var (
	errPrimaryNil               = errors.New("primary SQL connection is nil")
	errInvalidReplicaHostFormat = errors.New("invalid replica host format (expected host:port)")
	errDBNameRequired           = errors.New("DB_NAME is required")
)

// Config holds resolver configuration.
type Config struct {
	Strategy      StrategyType
	ReadFallback  bool
	MaxFailures   uint32
	TimeoutSec    uint32
	PrimaryRoutes []string
}

// ResolverProvider implements container.DBResolverProvider interface.
type ResolverProvider struct {
	resolver container.DB
	logger   any
	metrics  any
	tracer   trace.Tracer
	cfg      Config
	app      *gofr.App
}

// NewDBResolverProvider creates a new ResolverProvider.
func NewDBResolverProvider(app *gofr.App, cfg Config) *ResolverProvider {
	return &ResolverProvider{
		app: app,
		cfg: cfg,
	}
}

// UseLogger sets the logger.
func (p *ResolverProvider) UseLogger(logger any) {
	p.logger = logger
}

// UseMetrics sets the metrics.
func (p *ResolverProvider) UseMetrics(metrics any) {
	p.metrics = metrics
}

// UseTracer sets the tracer.
func (p *ResolverProvider) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		p.tracer = t
	}
}

// Connect initializes the resolver.
func (p *ResolverProvider) Connect() {
	// Get primary from app.
	primary := p.app.GetSQL()
	if primary == nil {
		if logger, ok := p.logger.(Logger); ok {
			logger.Error(errPrimaryNil)
		}

		return
	}

	// Convert logger and metrics to proper types.
	logger, loggerOk := p.logger.(Logger)
	metrics, metricsOk := p.metrics.(Metrics)

	if !loggerOk || !metricsOk {
		if loggerOk {
			logger.Errorf("Invalid logger or metrics type")
		}

		return
	}

	replicas := p.createAndValidateReplicas(logger, metrics)
	if replicas == nil {
		return
	}

	// Build primary routes map.
	primaryRoutesMap := make(map[string]bool)

	for _, route := range p.cfg.PrimaryRoutes {
		primaryRoutesMap[route] = true
	}

	strategy := getStrategy(p.cfg.Strategy)

	// Create resolver - single place!
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

// createAndValidateReplicas creates replicas and validates count.
func (p *ResolverProvider) createAndValidateReplicas(logger Logger, metrics Metrics) []container.DB {
	// Create replicas from config.
	replicas, err := connectReplicas(p.app.Config, logger, metrics)
	if err != nil {
		logger.Errorf("Failed to create replicas: %v", err)

		return nil
	}

	if len(replicas) == 0 {
		logger.Warn("No replicas configured - all operations will use primary")

		p.resolver = p.app.GetSQL()

		return nil
	}

	return replicas
}

// getStrategy returns the configured strategy.
func getStrategy(strategyType StrategyType) Strategy {
	switch strategyType {
	case StrategyRandom:
		return NewRandomStrategy()
	case StrategyRoundRobin:
		return NewRoundRobinStrategy()
	default:
		return NewRoundRobinStrategy()
	}
}

// GetResolver returns the underlying resolver.
func (p *ResolverProvider) GetResolver() container.DB {
	return p.resolver
}

// InitDBResolver - Complete initialization with middleware.
func InitDBResolver(app *gofr.App, cfg Config) error {
	provider := NewDBResolverProvider(app, cfg)

	//  Add middleware to inject HTTP context.
	app.UseMiddleware(createHTTPMiddleware())

	//  Add resolver (calls Connect() internally).
	app.AddDBResolver(provider)

	return nil
}

// createHTTPMiddleware injects HTTP method/path into context.
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

// connectReplicas creates replica connections from configuration.
func connectReplicas(cfg config.Config, logger Logger, metrics Metrics) ([]container.DB, error) {
	replicasStr := cfg.Get("DB_REPLICA_HOSTS")
	if replicasStr == "" {
		return nil, nil
	}

	replicaHosts := strings.Split(replicasStr, ",")
	replicas := make([]container.DB, 0, len(replicaHosts))

	user := cfg.Get("DB_REPLICA_USER")
	password := cfg.Get("DB_REPLICA_PASSWORD")

	for i, hostPort := range replicaHosts {
		hostPort = strings.TrimSpace(hostPort)
		if hostPort == "" {
			continue
		}

		parts := strings.Split(hostPort, ":")
		if len(parts) != expectedHostPortParts {
			return nil, fmt.Errorf("%w at index %d: %s", errInvalidReplicaHostFormat, i, hostPort)
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

// createReplicaConnection creates a single replica database connection.
func createReplicaConnection(cfg config.Config, host, port, user, password string, logger Logger, metrics Metrics) (container.DB, error) {
	dbName := cfg.Get("DB_NAME")

	if dbName == "" {
		return nil, errDBNameRequired
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

// replicaConfig wraps the main config and overrides specific values.
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

func getReplicaConfigInt(cfg config.Config, key string, fallback int) int {
	valStr := cfg.Get(key)
	if valStr == "" {
		return fallback
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return fallback
	}

	return val
}

func optimizedConnections(cfg config.Config, key string, minDefault, maxDefault, defaultVal, multiplier int) string {
	val := getReplicaConfigInt(cfg, key, defaultVal)
	if val <= 0 {
		return strconv.Itoa(defaultVal)
	}

	optimized := val * multiplier

	if optimized < minDefault {
		optimized = minDefault
	}

	if optimized > maxDefault {
		optimized = maxDefault
	}

	return strconv.Itoa(optimized)
}

func optimizedIdleConnections(cfg config.Config) string {
	// preserves previous behavior: read replica idle config, clamp to min/max
	return optimizedConnections(
		cfg,
		"DB_REPLICA_MAX_IDLE_CONNECTIONS",
		minIdleReplicaDefault,
		maxIdleReplicaCapDefault,
		minIdleReplicaDefault,
		1,
	)
}

func optimizedOpenConnections(cfg config.Config) string {
	// preserves previous behavior: read replica open config, clamp to min/max
	return optimizedConnections(
		cfg,
		"DB_REPLICA_MAX_OPEN_CONNECTIONS",
		minOpenReplicaDefault,
		maxOpenReplicaCapDefault,
		minOpenReplicaDefault,
		1,
	)
}
