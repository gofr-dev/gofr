package nats

import (
	"time"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const batchSize = 100

// Config defines the Client configuration.
type Config struct {
	Server      string
	CredsFile   string
	Stream      StreamConfig
	Consumer    string
	MaxWait     time.Duration
	MaxPullWait int
}

// StreamConfig holds stream settings for NATS JetStream.
type StreamConfig struct {
	Stream     string
	Subjects   []string
	MaxDeliver int
	MaxWait    time.Duration
	MaxBytes   int64
}

// New creates a new Client.
func New(cfg *Config, logger pubsub.Logger) *PubSubWrapper {
	if cfg == nil {
		cfg = &Config{}
	}

	client := &Client{
		Config:     cfg,
		subManager: newSubscriptionManager(batchSize),
		logger:     logger,
	}

	return &PubSubWrapper{Client: client}
}

// ValidateConfigs validates the configuration for NATS JetStream.
func ValidateConfigs(conf *Config) error {
	if conf.Server == "" {
		return errServerNotProvided
	}

	if len(conf.Stream.Subjects) == 0 {
		return errSubjectsNotProvided
	}

	if conf.Consumer == "" {
		return errConsumerNotProvided
	}

	return nil
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		MaxWait:     5 * time.Second,
		MaxPullWait: 10,
		Stream: StreamConfig{
			MaxDeliver: 3,
			MaxWait:    30 * time.Second,
		},
	}
}
