package nats

import "errors"

var (
	// Client Errors.
	errServerNotProvided       = errors.New("client server address not provided")
	errSubjectsNotProvided     = errors.New("subjects not provided")
	errConsumerNotProvided     = errors.New("consumer name not provided")
	errConsumerCreationError   = errors.New("consumer creation error")
	errFailedToDeleteStream    = errors.New("failed to delete stream")
	errPublishError            = errors.New("publish error")
	errJetStreamNotConfigured  = errors.New("jetStream is not configured")
	errJetStreamCreationFailed = errors.New("jetStream creation failed")
	errJetStream               = errors.New("jetStream error")
	errCreateStream            = errors.New("create stream error")
	errDeleteStream            = errors.New("delete stream error")
	errGetStream               = errors.New("get stream error")
	errCreateOrUpdateStream    = errors.New("create or update stream error")
	errHandlerError            = errors.New("handler error")
	errConnectionError         = errors.New("connection error")
	errSubscriptionError       = errors.New("subscription error")
)
