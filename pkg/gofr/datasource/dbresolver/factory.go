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
	expectedHostPortParts    = 2
)

var (
	errPrimaryNil               = errors.New("primary SQL connection is nil")
	errInvalidReplicaHostFormat = errors.New("invalid replica host format (expected host:port)")
	errDBNameRequired           = errors.New("DB_NAME is required")
	errEmptyCredentials         = errors.New("replica has empty credentials")
	errInvalidPort              = errors.New("invalid port for replica")
	errAllReplicasFailed        = errors.New("all replicas failed to connect")
)

// Config holds resolver configuration.
type Config struct {
	Strategy      StrategyType
	ReadFallback  bool
	MaxFailures   uint32
	TimeoutSec    uint32
	PrimaryRoutes []string
	Replicas      []ReplicaCredential
}

// ReplicaCredential stores credentials for a single replica.
type ReplicaCredential struct {
	Host     string `json:"host"` // Format: "hostname:port".
	User     string `json:"user"`
	Password string `json:"password"` // Supports commas and special chars.
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
func NewDBResolverProvider(app *gofr.App, cfg *Config) *ResolverProvider {
	return &ResolverProvider{
		app: app,
		cfg: *cfg,
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
func (p *ResolverProvider) createAndValidateReplicas(logger Logger, metrics Metrics) []container.DB {
	// Pass Config to connectReplicas
	replicas, err := connectReplicas(&p.cfg, p.app.Config, logger, metrics)
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
func InitDBResolver(app *gofr.App, cfg *Config) error {
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

// connectReplicas creates replica connections from Config.Replicas.
func connectReplicas(cfg *Config, appCfg config.Config, logger Logger, metrics Metrics) ([]container.DB, error) {
	if len(cfg.Replicas) == 0 {
		return nil, nil
	}

	replicas := make([]container.DB, 0, len(cfg.Replicas))

	var failedReplicas []string

	for i, cred := range cfg.Replicas {
		if err := validateReplicaCredentials(cred, i); err != nil {
			return nil, err
		}

		host, port, err := parseReplicaHost(cred.Host, i)
		if err != nil {
			return nil, err
		}

		replica, err := createReplicaConnection(appCfg, host, port, cred.User, cred.Password, logger, metrics)
		if err != nil {
			logger.Warnf("Failed to connect to replica #%d (%s): %v", i+1, cred.Host, err)

			failedReplicas = append(failedReplicas, cred.Host)

			continue // Skip failed replica instead of failing completely
		}

		replicas = append(replicas, replica)

		logger.Logf("Created DB replica connection to %s", cred.Host)
	}

	if len(replicas) == 0 {
		return nil, fmt.Errorf("%w (%d total)", errAllReplicasFailed, len(cfg.Replicas))
	}

	if len(failedReplicas) > 0 {
		logger.Warnf("%d/%d replicas failed: %v", len(failedReplicas), len(cfg.Replicas), failedReplicas)
	}

	return replicas, nil
}

// validateReplicaCredentials checks if all required fields are present.
func validateReplicaCredentials(cred ReplicaCredential, index int) error {
	if cred.Host == "" || cred.User == "" || cred.Password == "" {
		return fmt.Errorf("%w at index %d", errEmptyCredentials, index)
	}

	return nil
}

// parseReplicaHost splits host:port and validates the format.
func parseReplicaHost(host string, index int) (validatedHost, validatedPort string, err error) {
	parts := strings.Split(host, ":")
	if len(parts) != expectedHostPortParts {
		return "", "", fmt.Errorf("%w at index %d: %s", errInvalidReplicaHostFormat, index, host)
	}

	hostname := strings.TrimSpace(parts[0])
	port := strings.TrimSpace(parts[1])

	// Validate port is numeric
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", fmt.Errorf("%w at index %d: %s", errInvalidPort, index, port)
	}

	return hostname, port, nil
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
