package testutil

type mockConfig struct {
	conf map[string]string
}

type Config interface {
	Get(string) string
	GetOrDefault(string, string) string
}

func NewMockConfig(configMap map[string]string) Config {
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
