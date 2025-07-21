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

// StreamConfig holds stream settings for NATS jStream.
type StreamConfig struct {
	Stream     string
	Subjects   []string
	MaxDeliver int
	MaxWait    time.Duration
	MaxBytes   int64
	Storage    string
	Retention  string
	MaxAge     time.Duration
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

// validateConfigs validates the configuration for NATS jStream.
func validateConfigs(conf *Config) error {
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
