package config

// MockConfig is a mock type that can be used for testing purposes wherever GoDotEnvProvider's methods are called
type MockConfig struct {
	Data map[string]string
}

// Get retrieves the value of a configuration parameter by its key. If the key exists in the map, it returns the
// corresponding value else it returns the empty string.
func (m *MockConfig) Get(key string) string {
	return m.Data[key]
}

// GetOrDefault retrieves the value of a configuration parameter by its key. If the key exists in the map, it returns
// the corresponding value else it returns the specified defaultValue.
func (m *MockConfig) GetOrDefault(key, defaultValue string) string {
	v, ok := m.Data[key]
	if ok {
		return v
	}

	return defaultValue
}
