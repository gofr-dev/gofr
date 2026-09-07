package mcp

import (
	"context"

	"gofr.dev/pkg/gofr/ai"
)

const (
	defaultName    = "gofr-mcp"
	defaultVersion = "1.0.0"
)

// Hook runs before a tool is called; a non-nil error aborts the call with a JSON-RPC error.
type Hook func(ctx context.Context, spec ai.ToolSpec) error

type options struct {
	name    string
	version string
	hook    Hook
}

// Option configures a Server.
type Option func(*options)

// WithServerInfo sets the name and version reported in the initialize result.
func WithServerInfo(name, version string) Option {
	return func(o *options) {
		o.name = name
		o.version = version
	}
}

// WithHook registers a pre-call hook invoked with the matching tool spec before every tool call.
func WithHook(h Hook) Option {
	return func(o *options) {
		o.hook = h
	}
}
