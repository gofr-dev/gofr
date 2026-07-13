package llm

import "errors"

var (
	errNotConnected     = errors.New("llm: client not connected, call Connect first")
	errRequestFailed    = errors.New("llm: request failed")
	errUnexpectedStatus = errors.New("llm: unexpected status code")
	errEncodeRequest    = errors.New("llm: failed to encode request")
	errDecodeResponse   = errors.New("llm: failed to decode response")
	errStreamRead       = errors.New("llm: stream read failed")
	errProvider         = errors.New("llm: provider returned an error")
	errUnknownProvider  = errors.New("llm: unknown provider and no BaseURL set")
	errStreamToolsUnsup = errors.New("llm: streaming with tools is not supported")
)
