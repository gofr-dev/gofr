package rbac

import (
	"net/http"
)

// Provider is the RBAC provider implementation.
// Provider implements gofr.RBACProvider interface.
type Provider struct {
	config *Config // Store the loaded config
	logger any     // Store the logger (set via UseLogger)
}

// NewProvider creates a new RBAC provider.
//
// Example:
//
//	provider := rbac.NewProvider()
//	app.EnableRBAC(provider, "configs/rbac.json")
func NewProvider() *Provider {
	return &Provider{}
}

// UseLogger sets the logger for the provider.
// This is called automatically by EnableRBAC - users don't need to configure this.
func (p *Provider) UseLogger(logger any) {
	p.logger = logger
}

// LoadPermissions loads RBAC configuration from a file and stores it in the provider.
func (p *Provider) LoadPermissions(file string) (any, error) {
	config, err := LoadPermissions(file)
	if err != nil {
		return nil, err
	}

	p.config = config

	// Set logger on config if available (automatic audit logging)
	if p.logger != nil {
		config.SetLogger(p.logger)
	}

	return config, nil
}

// GetMiddleware returns the middleware function for the given config.
// All authorization is handled via unified config (Roles and Endpoints).
func (p *Provider) GetMiddleware(config any) func(http.Handler) http.Handler {
	rbacConfig, ok := config.(*Config)
	if !ok {
		// If config is not *Config, return passthrough middleware
		return func(handler http.Handler) http.Handler {
			return handler
		}
	}

	return Middleware(rbacConfig)
}
