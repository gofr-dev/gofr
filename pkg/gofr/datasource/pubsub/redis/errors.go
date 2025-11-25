package redis

import "errors"

var (
	errClientNotConnected     = errors.New("redis client not connected")
	errEmptyTopicName         = errors.New("topic name cannot be empty")
	errAddrNotProvided        = errors.New("redis address not provided")
	errPublisherNotConfigured = errors.New("redis publisher not configured")
	errInvalidDB              = errors.New("database number must be non-negative")
	errFailedToParseCACert    = errors.New("failed to parse CA certificate")
	errPubSubConnectionFailed = errors.New("failed to create PubSub connection for query")
	errPubSubChannelFailed    = errors.New("failed to get channel from PubSub for query")
)
