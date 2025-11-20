package config

// Subscriber represents any component that can be updated at runtime.
type Subscriber interface {
	UpdateConfig(config map[string]any)
}

// Provider represents a runtime config provider.
type Provider interface {
	Register(c Subscriber)
	Start()
}
