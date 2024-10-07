package nats

import (
	"time"
)

// Config defines the Client configuration.
type Config struct {
	Server      string
	CredsFile   string
	Stream      StreamConfig
	Consumer    string
	MaxWait     time.Duration
	MaxPullWait int
	BatchSize   int
}

// StreamConfig holds stream settings for NATS JetStream.
type StreamConfig struct {
	Stream     string
	Subjects   []string
	MaxDeliver int
	MaxWait    time.Duration
}

// New creates a new Client.
func New(cfg *Config) *PubSubWrapper {
	if cfg == nil {
		cfg = &Config{}
	}

	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100 // Default batch size
	}

	client := &Client{
		Config:     cfg,
		subManager: NewSubscriptionManager(cfg.BatchSize),
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
		BatchSize:   100,
		Stream: StreamConfig{
			MaxDeliver: 3,
			MaxWait:    30 * time.Second,
		},
	}
}
