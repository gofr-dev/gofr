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
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/logging"
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

	// RoleExtractorFunc extracts the user's role from the HTTP request
	// This function is called for each request to determine the user's role
	// The container (if available) is passed as the first argument in args
	// Container is only provided when using database-based role extraction
	// For header-based or JWT-based RBAC, args will be empty
	// Users can access container.SQL, container.Redis, etc. to use datasources
	// Example (header-based - no container needed):
	//   RoleExtractorFunc: func(req *http.Request, args ...any) (string, error) {
	//       return req.Header.Get("X-User-Role"), nil
	//   }
	// Example (database-based - container provided):
	//   RoleExtractorFunc: func(req *http.Request, args ...any) (string, error) {
	//       if len(args) > 0 {
	//           if cntr, ok := args[0].(*container.Container); ok && cntr != nil {
	//               // Use container.SQL.QueryRowContext(...) to query database
	//               var role string
	//               err := cntr.SQL.QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	//               return role, err
	//           }
	//       }
	//       return "", fmt.Errorf("database not available")
	//   }
	RoleExtractorFunc func(req *http.Request, args ...any) (string, error)

	// OverRides allows bypassing authorization for specific routes
	// Example: "/health": true (allows access without role check)
	OverRides map[string]bool `json:"overrides,omitempty" yaml:"overrides,omitempty"`

	// DefaultRole is used when no role can be extracted
	// If empty, missing role results in unauthorized error
	DefaultRole string `json:"defaultRole,omitempty" yaml:"defaultRole,omitempty"`

	// ErrorHandler is called when authorization fails
	// If nil, default error response is sent
	ErrorHandler func(w http.ResponseWriter, r *http.Request, role, route string, err error)

	// PermissionConfig enables permission-based access control
	// If set, permissions are checked instead of (or in addition to) roles
	PermissionConfig *PermissionConfig `json:"permissions,omitempty" yaml:"permissions,omitempty"`

	// EnablePermissions enables permission-based checks
	// If true, permission checks are performed in addition to role checks
	EnablePermissions bool `json:"enablePermissions,omitempty" yaml:"enablePermissions,omitempty"`

	// RoleHierarchy defines role inheritance relationships
	// Example: "admin": ["editor", "author", "viewer"]
	RoleHierarchy map[string][]string `json:"roleHierarchy,omitempty" yaml:"roleHierarchy,omitempty"`

	// Logger is the GoFr logger instance for audit logging
	// If nil, audit logging will be skipped
	// Audit logging is automatically performed using GoFr's logger when Logger is set
	Logger logging.Logger `json:"-" yaml:"-"`

	// RequiresContainer indicates if the RoleExtractorFunc needs access to the container
	// This flag determines whether the container is passed to RoleExtractorFunc
	// Set to true for database-based role extraction, false for header/JWT-based extraction
	// When false, the container is not passed to middleware, improving security and clarity
	RequiresContainer bool `json:"-" yaml:"-"`
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

// NewConfigLoader creates a new config loader.
func NewConfigLoader(path string, _ time.Duration) (*ConfigLoader, error) {
	return NewConfigLoaderWithLogger(path, nil)
}

// NewConfigLoaderWithLogger creates a new ConfigLoader with a logger for error reporting.
func NewConfigLoaderWithLogger(path string, logger logging.Logger) (*ConfigLoader, error) {
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
func (l *ConfigLoader) GetConfig() gofr.RBACConfig {
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
func (c *Config) GetRoleExtractorFunc() gofr.RoleExtractor {
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

// GetRequiresContainer returns whether container access is needed.
func (c *Config) GetRequiresContainer() bool {
	return c.RequiresContainer
}

// SetRoleExtractorFunc sets the role extractor function.
func (c *Config) SetRoleExtractorFunc(extractor gofr.RoleExtractor) {
	c.RoleExtractorFunc = extractor
}

// SetLogger sets the logger instance.
func (c *Config) SetLogger(logger any) {
	if l, ok := logger.(logging.Logger); ok {
		c.Logger = l
	}
}

// SetRequiresContainer sets whether container access is needed.
func (c *Config) SetRequiresContainer(required bool) {
	c.RequiresContainer = required
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
