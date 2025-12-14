package rbac

import (
	"errors"
	"net/http"
	"os"

	"go.opentelemetry.io/otel/trace"
)

var (
	// ErrConfigPathNotSet is returned when config path is not set.
	ErrConfigPathNotSet = errors.New("config path not set")
)

const (
	// Default RBAC config paths (tried in order).
	defaultRBACJSONPath = "configs/rbac.json"
	defaultRBACYAMLPath = "configs/rbac.yaml"
	defaultRBACYMLPath  = "configs/rbac.yml"
)

// Provider is the RBAC provider implementation.
// Provider implements gofr.RBACProvider interface.
type Provider struct {
	configPath string       // Store the config file path
	config     *Config      // Store the loaded config
	logger     Logger       // Store the logger (set via UseLogger)
	metrics    Metrics      // Store the metrics (set via UseMetrics)
	tracer     trace.Tracer // Store the tracer (set via UseTracer)
}

// NewProvider creates a new RBAC provider with the config file path.
// If configPath is empty, it will try default paths: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml
//
// Example:
//
//	provider := rbac.NewProvider("configs/rbac.json")
//	app.EnableRBAC(provider)
func NewProvider(configPath string) *Provider {
	// If empty, resolve default paths
	if configPath == "" {
		configPath = resolveRBACConfigPath("")
	}

	return &Provider{
		configPath: configPath,
	}
}

// resolveRBACConfigPath resolves the RBAC config file path.
func resolveRBACConfigPath(configFile string) string {
	// If custom path provided, use it
	if configFile != "" {
		return configFile
	}

	// Try default paths in order
	defaultPaths := []string{
		defaultRBACJSONPath,
		defaultRBACYAMLPath,
		defaultRBACYMLPath,
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// UseLogger sets the logger for the provider which asserts the Logger interface.
// This is called automatically by EnableRBAC - users don't need to configure this.
func (p *Provider) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		p.logger = l
	}
}

// UseMetrics sets the metrics for the provider which asserts the Metrics interface.
// This is called automatically by EnableRBAC - users don't need to configure this.
func (p *Provider) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		p.metrics = m
		p.registerMetrics()
	}
}

func (p *Provider) registerMetrics() {
	buckets := []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1}
	p.metrics.NewHistogram(
		"rbac_authorization_duration",
		"Duration of RBAC authorization checks",
		buckets...,
	)
	p.metrics.NewCounter("rbac_authorization_decisions", "Number of RBAC authorization decisions")
	p.metrics.NewCounter("rbac_role_extraction_failures", "Number of failed role extractions")
}

// UseTracer sets the tracer for the provider.
// This is called automatically by EnableRBAC - users don't need to configure this.
func (p *Provider) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		p.tracer = t
	}
}

// LoadPermissions loads RBAC configuration from the stored config path and stores it in the provider.
func (p *Provider) LoadPermissions() error {
	if p.configPath == "" {
		return ErrConfigPathNotSet
	}

	config, err := LoadPermissions(p.configPath)
	if err != nil {
		return err
	}

	p.config = config

	// Set logger on config if available (automatic audit logging)
	if p.logger != nil {
		config.Logger = p.logger
	}

	// Set metrics on config if available
	if p.metrics != nil {
		config.Metrics = p.metrics
	}

	// Set tracer on config if available
	if p.tracer != nil {
		config.Tracer = p.tracer
	}

	return nil
}

// RBACMiddleware returns the middleware function using the stored config.
// All authorization is handled via unified config (Roles and Endpoints).
func (p *Provider) RBACMiddleware() func(http.Handler) http.Handler {
	if p.config == nil {
		// If config is not loaded, return passthrough middleware
		return func(handler http.Handler) http.Handler {
			return handler
		}
	}

	return Middleware(p.config)
}
