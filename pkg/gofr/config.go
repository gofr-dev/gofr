package gofr

// Config provides the functionality to read configurations defined for the application
type Config interface {
	// Get returns the config value for a particular config key
	Get(string) string
	// GetOrDefault returns the config value for a particular config key or returns a default value if not present
	GetOrDefault(string, string) string
}
