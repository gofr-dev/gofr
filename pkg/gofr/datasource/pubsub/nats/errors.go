package nats

import "errors"

var (
	// NATS Errors.
	ErrFailedToCreateConsumer = errors.New("failed to create or attach consumer")
	errPublisherNotConfigured = errors.New("can't publish message: publisher not configured or stream is empty")
	errPublish                = errors.New("failed to publish message to NATS JetStream")
	errSubscribe              = errors.New("subscribe error")
	ErrNoMessagesReceived     = errors.New("no messages received")
	ErrServerNotProvided      = errors.New("NATS server address not provided")
	errNATSConnection         = errors.New("failed to connect to NATS server")

	// NATS JetStream Errors.
	ErrConsumerNotProvided = errors.New("consumer name not provided")
	ErrStreamNotProvided   = errors.New("stream name not provided")
	errJetStream           = errors.New("JetStream error")
	errSubjectsNotProvided = errors.New("subjects not provided")
)
