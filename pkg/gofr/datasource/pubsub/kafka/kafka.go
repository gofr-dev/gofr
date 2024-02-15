package kafka

import (
	"context"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var errPublisherNotConfigured = errors.New("can't publish message. Publisher not configured or topic is empty")

type Config struct {
	Broker          string
	Partition       int
	ConsumerGroupID string
	Topic           string
}

type kafkaClient struct {
	dialer *kafka.Dialer
	writer *kafka.Writer
	reader *kafka.Reader
	logger pubsub.Logger
	config Config
}

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger) *kafkaClient {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{conf.Broker},
		Dialer:  dialer,
	})

	return &kafkaClient{
		config: conf,
		dialer: dialer,
		logger: logger,
		writer: writer,
	}
}
func (k *kafkaClient) Publish(ctx context.Context, topic string, message []byte) error {
	if k.writer == nil || topic == "" {
		return errPublisherNotConfigured
	}

	err := k.writer.WriteMessages(ctx,
		kafka.Message{
			Topic: topic,
			Value: message,
		},
	)

	if err != nil {
		k.logger.Error("failed to publish message to kafka broker")
		return err
	}

	return nil
}

func (k *kafkaClient) Subscribe(ctx context.Context, topic string) (pubsub.Message, error) {
	if k.reader == nil {
		reader := kafka.NewReader(kafka.ReaderConfig{
			GroupID:  k.config.ConsumerGroupID,
			Brokers:  []string{k.config.Broker},
			Topic:    topic,
			MinBytes: 10e3,
			MaxBytes: 10e6,
			Dialer:   k.dialer,
		})

		k.reader = reader
	}

	// Read a single message from the topic
	msg, err := k.reader.ReadMessage(ctx)
	if err != nil {
		k.logger.Errorf("failed to read message from Kafka topic %s: %v", topic, err)
		return pubsub.Message{}, err
	}

	return pubsub.Message{
		Value: msg.Value,
		Topic: topic,
	}, nil
}

func (k *kafkaClient) Commit(ctx context.Context, msg pubsub.Message) error {
	err := k.reader.CommitMessages(ctx, kafka.Message{
		Topic:     msg.Topic,
		Partition: k.config.Partition,
	})
	if err != nil {
		k.logger.Errorf("failed to commit message from topic %s: %w", msg.Topic, err)
		return err
	}

	return nil
}

func (k *kafkaClient) Close() error {
	err := k.writer.Close()
	if err != nil {
		k.logger.Errorf("failed to close Kafka writer: %v", err)
		return err
	}

	return nil
}
