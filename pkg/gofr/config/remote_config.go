package config

// RemoteConfigurable represents any component that can be updated at runtime
type RemoteConfigurable interface {
	UpdateConfig(config map[string]any)
}

// RemoteConfiguration represents a runtime config provider
type RemoteConfiguration interface {
	Register(c RemoteConfigurable)
	Start()
}
