package config

type TestConfig map[string]string

func (c TestConfig) Get(key string) string {
	return c[key]
}

func (c TestConfig) GetOrDefault(key, defaultValue string) string {
	if value, ok := c[key]; ok {
		return value
	}

	return defaultValue
}
