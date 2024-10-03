package nats

import "errors"

var (
	// Client Errors.
	errServerNotProvided          = errors.New("client server address not provided")
	errSubjectsNotProvided        = errors.New("subjects not provided")
	errConsumerNotProvided        = errors.New("consumer name not provided")
	errFailedToCreateStream       = errors.New("failed to create stream")
	errFailedToDeleteStream       = errors.New("failed to delete stream")
	errFailedToCreateConsumer     = errors.New("failed to create consumer")
	errPublishError               = errors.New("publish error")
	errFailedCreateOrUpdateStream = errors.New("create or update stream error")
	errJetStreamNotConfigured     = errors.New("JetStream is not configured")
	errJetStream                  = errors.New("JetStream error")
	errNATSConnNil                = errors.New("NATS connection is nil")

	// Message Errors.
	errHandlerError = errors.New("handler error")
)
