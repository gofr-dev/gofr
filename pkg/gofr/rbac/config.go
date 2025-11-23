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

	"gopkg.in/yaml.v3"

	"gofr.dev/pkg/gofr/logging"
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

	// EnableCache enables role caching for performance
	EnableCache bool `json:"enableCache,omitempty" yaml:"enableCache,omitempty"`

	// CacheTTL is the cache time-to-live for roles
	CacheTTL time.Duration `json:"cacheTTL,omitempty" yaml:"cacheTTL,omitempty"`

	// Logger is the GoFr logger instance for audit logging and config reload errors
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

// ConfigLoader manages loading and reloading of RBAC configuration.
type ConfigLoader struct {
	path     string
	config   *Config
	mu       sync.RWMutex
	lastMod  time.Time
	interval time.Duration
	stopCh   chan struct{}
	logger   logging.Logger
}

// NewConfigLoader creates a new config loader with hot-reload capability.
// If reloadInterval is 0, hot-reload is disabled.
func NewConfigLoader(path string, reloadInterval time.Duration) (*ConfigLoader, error) {
	return NewConfigLoaderWithLogger(path, reloadInterval, nil)
}

// NewConfigLoaderWithLogger creates a new ConfigLoader with a logger for error reporting.
func NewConfigLoaderWithLogger(path string, reloadInterval time.Duration, logger logging.Logger) (*ConfigLoader, error) {
	config, err := LoadPermissions(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	loader := &ConfigLoader{
		path:     path,
		config:   config,
		lastMod:  info.ModTime(),
		interval: reloadInterval,
		stopCh:   make(chan struct{}),
		logger:   logger,
	}

	// Start hot-reload if interval is set
	if reloadInterval > 0 {
		go loader.reloadLoop()
	}

	return loader, nil
}

// GetConfig returns the current configuration (thread-safe).
func (l *ConfigLoader) GetConfig() *Config {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.config
}

// Reload manually reloads the configuration from file.
func (l *ConfigLoader) Reload() error {
	config, err := LoadPermissions(l.path)
	if err != nil {
		return err
	}

	info, err := os.Stat(l.path)
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.config = config
	l.lastMod = info.ModTime()
	l.mu.Unlock()

	return nil
}

// reloadLoop periodically checks for file changes and reloads if needed.
func (l *ConfigLoader) reloadLoop() {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			info, err := os.Stat(l.path)
			if err != nil {
				continue
			}

			if info.ModTime().After(l.lastMod) {
				if err := l.Reload(); err != nil {
					// Log error but continue
					if l.logger != nil {
						l.logger.Errorf("[RBAC] Failed to reload config: %v", err)
					}
				}
			}
		case <-l.stopCh:
			return
		}
	}
}

// Stop stops the hot-reload loop.
func (l *ConfigLoader) Stop() {
	close(l.stopCh)
}

// RoleExtractor is a type alias for role extraction functions.
type RoleExtractor func(req *http.Request, args ...any) (string, error)

// JWTConfig holds configuration for JWT-based role extraction.
type JWTConfig struct {
	// RoleClaim is the path to the role claim in JWT
	// Examples: "role", "roles[0]", "permissions.role"
	RoleClaim string

	// JWKSEndpoint is the endpoint to fetch JWKS (if needed)
	JWKSEndpoint string

	// RefreshInterval is how often to refresh JWKS
	RefreshInterval time.Duration
}

// DBConfig holds configuration for database-based role extraction.
type DBConfig struct {
	// UserIDExtractor extracts user ID from request
	UserIDExtractor func(req *http.Request) string

	// RoleQuery is the SQL query to fetch role
	// Should return a single string value (the role)
	// Example: "SELECT role FROM users WHERE id = ?"
	RoleQuery string

	// CacheTTL is the cache time-to-live for roles
	CacheTTL time.Duration
}
