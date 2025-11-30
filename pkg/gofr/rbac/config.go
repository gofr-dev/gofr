package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	// errUnsupportedFormat is returned when the config file format is not supported.
	errUnsupportedFormat = errors.New("unsupported config file format")
)

// Config represents the RBAC configuration structure.
type Config struct {
	// RouteWithPermissions maps route patterns to allowed roles
	// Example: "/api/users": ["admin", "editor"]
	RouteWithPermissions map[string][]string `json:"route" yaml:"route"`

	// RoleHeader specifies the HTTP header key for role extraction
	// Example: "X-User-Role"
	// If set, automatically creates a header-based role extractor
	RoleHeader string `json:"roleHeader,omitempty" yaml:"roleHeader,omitempty"`

	// RoleExtractorFunc extracts the user's role from the HTTP request
	// This function is called for each request to determine the user's role
	// Args will be empty for header/JWT-based extraction
	RoleExtractorFunc func(req *http.Request, args ...any) (string, error)

	// OverRides allows bypassing authorization for specific routes
	// Example: "/health": true (allows access without role check)
	OverRides map[string]bool `json:"overrides,omitempty" yaml:"overrides,omitempty"`

	// DefaultRole is used when no role can be extracted
	// If empty, missing role results in unauthorized error
	// ⚠️ Security Warning: Using defaultRole can be a security flaw if not carefully considered.
	// Only use for internal services in trusted networks or development/testing environments.
	DefaultRole string `json:"defaultRole,omitempty" yaml:"defaultRole,omitempty"`

	// ErrorHandler is called when authorization fails
	// If nil, default error response is sent
	ErrorHandler func(w http.ResponseWriter, r *http.Request, role, route string, err error)

	// PermissionConfig enables permission-based access control
	// If set, permissions are checked instead of (or in addition to) roles
	PermissionConfig *PermissionConfig `json:"permissions,omitempty" yaml:"permissions,omitempty"`

	// EnablePermissions enables permission-based checks
	// Auto-detected from PermissionConfig presence, but can be set explicitly
	EnablePermissions bool `json:"-" yaml:"-"`

	// RoleHierarchy defines role inheritance relationships
	// Example: "admin": ["editor", "author", "viewer"]
	RoleHierarchy map[string][]string `json:"roleHierarchy,omitempty" yaml:"roleHierarchy,omitempty"`

	// Logger is the logger instance for audit logging
	// If nil, audit logging will be skipped
	// Audit logging is automatically performed when Logger is set
	Logger Logger `json:"-" yaml:"-"`
}

// LoadPermissions loads RBAC configuration from a JSON or YAML file.
// The file format is automatically detected based on the file extension.
// Supported formats: .json, .yaml, .yml.
func LoadPermissions(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read RBAC config file %s: %w", path, err)
	}

	var config Config

	// Detect file format by extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", path, err)
		}
	case ".json", "":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml): %w", ext, errUnsupportedFormat)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	// Initialize empty maps if not present
	if config.RouteWithPermissions == nil {
		config.RouteWithPermissions = make(map[string][]string)
	}

	if config.OverRides == nil {
		config.OverRides = make(map[string]bool)
	}

	// Auto-detect permissions if PermissionConfig is set
	if config.PermissionConfig != nil {
		config.EnablePermissions = true
		// Compile regex patterns in route rules
		if err := config.PermissionConfig.CompileRoutePermissionRules(); err != nil {
			return nil, fmt.Errorf("failed to compile route permission rules: %w", err)
		}
	}

	return &config, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over file-based configuration.
func applyEnvOverrides(config *Config) {
	// Override default role from environment
	if defaultRole := os.Getenv("RBAC_DEFAULT_ROLE"); defaultRole != "" {
		config.DefaultRole = defaultRole
	}

	// Override specific routes from environment
	// Format: RBAC_ROUTE_<ROUTE_PATH>=role1,role2,role3
	// Example: RBAC_ROUTE_/api/users=admin,editor
	for _, env := range os.Environ() {
		key, value, found := strings.Cut(env, "=")
		if !found {
			continue
		}

		if strings.HasPrefix(key, "RBAC_ROUTE_") {
			applyRouteOverride(config, key, value)
		}

		if strings.HasPrefix(key, "RBAC_OVERRIDE_") {
			applyOverrideFlag(config, key, value)
		}
	}
}

// applyRouteOverride applies a route override from environment variable.
func applyRouteOverride(config *Config, key, value string) {
	route := strings.TrimPrefix(key, "RBAC_ROUTE_")
	route = strings.ReplaceAll(route, "_", "/")
	roles := strings.Split(value, ",")

	// Trim whitespace from roles
	for i, role := range roles {
		roles[i] = strings.TrimSpace(role)
	}

	if config.RouteWithPermissions == nil {
		config.RouteWithPermissions = make(map[string][]string)
	}

	config.RouteWithPermissions[route] = roles
}

// applyOverrideFlag applies an override flag from environment variable.
func applyOverrideFlag(config *Config, key, value string) {
	route := strings.TrimPrefix(key, "RBAC_OVERRIDE_")
	route = strings.ReplaceAll(route, "_", "/")

	if strings.EqualFold(value, "true") || value == "1" {
		if config.OverRides == nil {
			config.OverRides = make(map[string]bool)
		}

		config.OverRides[route] = true
	}
}

// ConfigLoader manages loading of RBAC configuration.
type ConfigLoader struct {
	config *Config
	mu     sync.RWMutex
}

// NewConfigLoaderWithLogger creates a new ConfigLoader with a logger for error reporting.
func NewConfigLoaderWithLogger(path string, logger Logger) (*ConfigLoader, error) {
	config, err := LoadPermissions(path)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		config.Logger = logger
	}

	loader := &ConfigLoader{
		config: config,
	}

	return loader, nil
}

// GetConfig returns the current configuration (thread-safe).
func (l *ConfigLoader) GetConfig() RBACConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.config
}

// Implement RBACConfig interface methods

// GetRouteWithPermissions returns the route-to-roles mapping.
func (c *Config) GetRouteWithPermissions() map[string][]string {
	return c.RouteWithPermissions
}

// GetRoleExtractorFunc returns the role extractor function.
func (c *Config) GetRoleExtractorFunc() RoleExtractor {
	return c.RoleExtractorFunc
}

// GetPermissionConfig returns permission configuration if enabled.
func (c *Config) GetPermissionConfig() any {
	// Return as any to match interface, will be type-asserted by callers
	return c.PermissionConfig
}

// GetOverRides returns route overrides.
func (c *Config) GetOverRides() map[string]bool {
	return c.OverRides
}

// GetLogger returns the logger instance.
func (c *Config) GetLogger() any {
	return c.Logger
}


// GetRoleHeader returns the role header key if configured.
func (c *Config) GetRoleHeader() string {
	return c.RoleHeader
}

// SetRoleExtractorFunc sets the role extractor function.
func (c *Config) SetRoleExtractorFunc(extractor RoleExtractor) {
	c.RoleExtractorFunc = extractor
}

// SetLogger sets the logger instance.
func (c *Config) SetLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.Logger = l
	}
}


// SetEnablePermissions enables permission-based access control.
func (c *Config) SetEnablePermissions(enabled bool) {
	c.EnablePermissions = enabled
}

// InitializeMaps initializes empty maps if not present.
func (c *Config) InitializeMaps() {
	if c.RouteWithPermissions == nil {
		c.RouteWithPermissions = make(map[string][]string)
	}

	if c.OverRides == nil {
		c.OverRides = make(map[string]bool)
	}

	if c.RoleHierarchy == nil {
		c.RoleHierarchy = make(map[string][]string)
	}
}
