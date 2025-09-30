package nats

import (
	"context"
	"errors"
	"strings"

	"github.com/nats-io/nats.go/jetstream"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// StreamManager is a manager for jStream streams.
type StreamManager struct {
	js     jetstream.JetStream
	logger pubsub.Logger
}

// newStreamManager creates a new StreamManager.
func newStreamManager(js jetstream.JetStream, logger pubsub.Logger) *StreamManager {
	return &StreamManager{
		js:     js,
		logger: logger,
	}
}

// CreateStream creates a new jStream stream.
func (sm *StreamManager) CreateStream(ctx context.Context, cfg *StreamConfig) error {
	jsCfg := jetstream.StreamConfig{
		Name:     cfg.Stream,
		Subjects: cfg.Subjects,
		MaxBytes: cfg.MaxBytes,
		MaxAge:   cfg.MaxAge,
	}

	if cfg.Storage != "" {
		if cfg.Storage == "file" {
			jsCfg.Storage = jetstream.FileStorage
		} else if cfg.Storage == "memory" {
			jsCfg.Storage = jetstream.MemoryStorage
		}
	}

	if cfg.Retention != "" {
		switch cfg.Retention {
		case "limits":
			jsCfg.Retention = jetstream.LimitsPolicy
		case "interest":
			jsCfg.Retention = jetstream.InterestPolicy
		case "workqueue":
			jsCfg.Retention = jetstream.WorkQueuePolicy
		}
	}

	_, err := sm.js.CreateStream(ctx, jsCfg)
	if err != nil {
		if strings.Contains(err.Error(), "stream name already in use") {
			return nil
		}

		sm.logger.Errorf("failed to create stream: %v", err)

		return err
	}

	return nil
}

// DeleteStream deletes a jStream stream.
func (sm *StreamManager) DeleteStream(ctx context.Context, name string) error {
	sm.logger.Debugf("deleting stream %s", name)

	err := sm.js.DeleteStream(ctx, name)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			sm.logger.Debugf("stream %s not found, considering delete successful", name)

			return nil // If the stream doesn't exist, we consider it a success
		}

		sm.logger.Errorf("failed to delete stream %s: %v", name, err)

		return err
	}

	sm.logger.Debugf("successfully deleted stream %s", name)

	return nil
}

// CreateOrUpdateStream creates or updates a jStream stream.
func (sm *StreamManager) CreateOrUpdateStream(ctx context.Context, cfg *jetstream.StreamConfig) (jetstream.Stream, error) {
	sm.logger.Debugf("creating or updating stream %s", cfg.Name)

	stream, err := sm.js.CreateOrUpdateStream(ctx, *cfg)
	if err != nil {
		sm.logger.Errorf("failed to create or update stream: %v", err)

		return nil, err
	}

	return stream, nil
}

// GetStream gets a jStream stream.
func (sm *StreamManager) GetStream(ctx context.Context, name string) (jetstream.Stream, error) {
	sm.logger.Debugf("getting stream %s", name)

	stream, err := sm.js.Stream(ctx, name)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			sm.logger.Debugf("stream %s not found", name)

			return nil, err
		}

		sm.logger.Errorf("failed to get stream %s: %v", name, err)

		return nil, err
	}

	return stream, nil
}
