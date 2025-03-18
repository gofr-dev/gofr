package config

type mockConfig struct {
	conf map[string]string
}

func NewMockConfig(configMap map[string]string) Config {
	if configMap == nil {
		configMap = make(map[string]string)
	}

	// setting telemetry false for running tests
	configMap["GOFR_TELEMETRY"] = "false"

	return &mockConfig{
		conf: configMap,
	}
}

func (m *mockConfig) Get(s string) string {
	return m.conf[s]
}

func (m *mockConfig) GetOrDefault(s, d string) string {
	res, ok := m.conf[s]
	if !ok {
		res = d
	}

	return res
}
