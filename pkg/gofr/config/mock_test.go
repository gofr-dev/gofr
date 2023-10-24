package config

import "testing"

func TestMockConfig_Get(t *testing.T) {
	const defaultValue = "test"

	m := MockConfig{
		Data: map[string]string{
			"key": "value",
		},
	}

	if m.Get("key") != "value" || m.Get("random") != "" {
		t.Error("Get not working")
	}

	if m.GetOrDefault("key", defaultValue) != "value" ||
		m.GetOrDefault("random", defaultValue) != defaultValue {
		t.Error("GetOrDefault not working")
	}
}
