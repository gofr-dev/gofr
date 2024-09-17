package nats

import "errors"

var (
	// NATS Errors.
	ErrConnectionStatus           = errors.New("unexpected NATS connection status")
	ErrServerNotProvided          = errors.New("NATS server address not provided")
	ErrSubjectsNotProvided        = errors.New("subjects not provided")
	ErrConsumerNotProvided        = errors.New("consumer name not provided")
	ErrEmbeddedNATSServerNotReady = errors.New("embedded NATS server not ready")
	ErrFailedToCreateStream       = errors.New("failed to create stream")
	ErrFailedToDeleteStream       = errors.New("failed to delete stream")
	ErrFailedToCreateConsumer     = errors.New("failed to create consumer")
	ErrConnectionFailed           = errors.New("connection failed")
	ErrPublishError               = errors.New("publish error")
	ErrFailedNatsClientCreation   = errors.New("NATS client creation failed")
	ErrFailedCreateOrUpdateStream = errors.New("create or update stream error")
	ErrJetStreamNotConfigured     = errors.New("JetStream is not configured")
	errJetStream                  = errors.New("JetStream error")

	// Message Errors.
	ErrHandlerError = errors.New("handler error")
)
