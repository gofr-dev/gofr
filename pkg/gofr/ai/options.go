package ai

// CallOptions carries per-call settings. It is exported so provider modules in other packages can
// read the applied options. New fields may be appended; existing ones never change meaning.
type CallOptions struct {
	Tools       []ToolSpec
	Temperature *float64
	MaxTokens   *int
}

// Option mutates CallOptions.
type Option func(*CallOptions)

// ApplyOptions folds opts onto a zero CallOptions and returns the result.
func ApplyOptions(opts ...Option) CallOptions {
	var c CallOptions

	for _, opt := range opts {
		if opt != nil {
			opt(&c)
		}
	}

	return c
}

// WithTools restricts a call to the given tool specs.
func WithTools(tools []ToolSpec) Option {
	return func(c *CallOptions) { c.Tools = tools }
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) Option {
	return func(c *CallOptions) { c.Temperature = &t }
}

// WithMaxTokens caps the tokens generated in the completion.
func WithMaxTokens(n int) Option {
	return func(c *CallOptions) { c.MaxTokens = &n }
}
