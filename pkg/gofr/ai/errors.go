package ai

import "errors"

var (
	// ErrStreamNotSupported is returned by Stream when the underlying model cannot stream.
	ErrStreamNotSupported = errors.New("streaming is not supported by this model")
	// ErrToolNotFound is returned by Tools.Call for an unknown tool name.
	ErrToolNotFound = errors.New("tool not found")
)
