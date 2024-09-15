package nats

import "errors"

var (
	// NATS Errors.
	ErrConnectionStatus           = errors.New("unexpected NATS connection status")
	ErrServerNotProvided          = errors.New("NATS server address not provided")
	ErrSubjectsNotProvided        = errors.New("subjects not provided")
	ErrJetStreamNotConfigured     = errors.New("JetStream is not configured")
	ErrConsumerNotProvided        = errors.New("consumer name not provided")
	ErrEmbeddedNATSServerNotReady = errors.New("embedded NATS server not ready")
	ErrFailedToCreateStream       = errors.New("failed to create stream")
	errJetStream                  = errors.New("JetStream error")
)
