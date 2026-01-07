package sqs

import "time"

// Config holds the configuration for the SQS client.
type Config struct {
	// Region is the AWS region where the SQS queue is located.
	// Example: "us-east-1", "eu-west-1"
	Region string

	// Endpoint is the custom endpoint URL for SQS.
	// This is useful for local development with LocalStack.
	// Example: "http://localhost:4566"
	// If empty, the default AWS endpoint will be used.
	Endpoint string

	// AccessKeyID is the AWS access key ID for authentication.
	// If empty, the SDK will use the default credential chain.
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key for authentication.
	// If empty, the SDK will use the default credential chain.
	SecretAccessKey string

	// SessionToken is the AWS session token for temporary credentials.
	// Optional - only needed when using temporary credentials.
	SessionToken string

	// QueueURL is the URL of the SQS queue.
	// If not provided, the client will attempt to get/create the queue URL from QueueName.
	// Example: "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue"
	QueueURL string

	// MaxMessages is the maximum number of messages to receive in a single request (1-10).
	// Default: 1
	MaxMessages int32

	// WaitTimeSeconds is the duration (in seconds) for which the call waits for a message to arrive.
	// This enables long polling. Default: 20 (maximum allowed by SQS)
	WaitTimeSeconds int32

	// VisibilityTimeout is the duration (in seconds) that a received message is hidden from subsequent retrieve requests.
	// Default: 30 seconds
	VisibilityTimeout int32

	// DelaySeconds is the length of time (in seconds) to delay a specific message. (0-900)
	// Default: 0
	DelaySeconds int32

	// RetryDuration is the duration to wait before retrying failed connection attempts.
	// Default: 5 seconds
	RetryDuration time.Duration
}

// setDefaults sets default values for the configuration.
func (c *Config) setDefaults() {
	if c.MaxMessages <= 0 || c.MaxMessages > 10 {
		c.MaxMessages = 1
	}

	if c.WaitTimeSeconds <= 0 || c.WaitTimeSeconds > 20 {
		c.WaitTimeSeconds = 20
	}

	if c.VisibilityTimeout <= 0 {
		c.VisibilityTimeout = 30
	}

	if c.DelaySeconds < 0 || c.DelaySeconds > 900 {
		c.DelaySeconds = 0
	}

	if c.RetryDuration <= 0 {
		c.RetryDuration = 5 * time.Second
	}
}
