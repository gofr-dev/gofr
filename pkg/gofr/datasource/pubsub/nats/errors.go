package nats

import "errors"

var (
	// Client Errors.
	errServerNotProvided       = errors.New("client server address not provided")
	errSubjectsNotProvided     = errors.New("subjects not provided")
	errConsumerNotProvided     = errors.New("consumer name not provided")
	errConsumerCreationError   = errors.New("consumer creation error")
	errFailedToDeleteStream    = errors.New("failed to delete stream")
	errFailedToCreateConsumer  = errors.New("failed to create consumer")
	errPublishError            = errors.New("publish error")
	errJetStreamNotConfigured  = errors.New("JetStream is not configured")
	errJetStreamCreationFailed = errors.New("JetStream creation failed")
	errJetStream               = errors.New("JetStream error")
	errCreateStream            = errors.New("create stream error")
	errDeleteStream            = errors.New("delete stream error")
	errGetStream               = errors.New("get stream error")
	errCreateOrUpdateStream    = errors.New("create or update stream error")
)
