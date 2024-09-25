package nats

import "errors"

var (
	// Client Errors.
	errConnectionStatus           = errors.New("unexpected Client connection status")
	errServerNotProvided          = errors.New("Client server address not provided")
	errSubjectsNotProvided        = errors.New("subjects not provided")
	errConsumerNotProvided        = errors.New("consumer name not provided")
	errFailedToCreateStream       = errors.New("failed to create stream")
	errFailedToDeleteStream       = errors.New("failed to delete stream")
	errFailedToCreateConsumer     = errors.New("failed to create consumer")
	errPublishError               = errors.New("publish error")
	errFailedCreateOrUpdateStream = errors.New("create or update stream error")
	errJetStreamNotConfigured     = errors.New("JetStream is not configured")
	errJetStream                  = errors.New("JetStream error")

	// Message Errors.
	errHandlerError = errors.New("handler error")
)
