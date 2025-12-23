package rbac

import "os"

const (
	// DefaultConfigPath is the default config path value (empty string).
	// When passed to ResolveRBACConfigPath, it will try default paths: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
	DefaultConfigPath = ""

	// Default RBAC config paths (tried in order).
	defaultRBACJSONPath = "configs/rbac.json"
	defaultRBACYAMLPath = "configs/rbac.yaml"
	defaultRBACYMLPath  = "configs/rbac.yml"
)

// ResolveRBACConfigPath resolves the RBAC config file path.
// If configFile is empty, tries default paths in order: configs/rbac.json, configs/rbac.yaml, configs/rbac.yml.
func ResolveRBACConfigPath(configFile string) string {
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
