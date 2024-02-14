package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type Config struct {
	Broker          string
	Partition       int
	Offset          int64
	ConsumerGroupID string
	Topic           string
}

type kafkaClient struct {
	writer *kafka.Writer
	logger pubsub.Logger
	config Config
}

func New(conf Config, logger pubsub.Logger) *kafkaClient {
	return &kafkaClient{
		config: conf,
		logger: logger,
		writer: &kafka.Writer{
			Addr:     kafka.TCP(conf.Broker),
			Balancer: &kafka.LeastBytes{},
		},
	}
}
func (k *kafkaClient) Publish(ctx context.Context, topic string, message []byte) error {
	if k.writer == nil || topic == "" {
		k.logger.Error("can't publish message. Publisher not configured or topic is empty")
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
	reader := kafka.NewReader(kafka.ReaderConfig{
		GroupID:  k.config.ConsumerGroupID,
		Brokers:  []string{k.config.Broker},
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	// Read a single message from the topic
	msgValue, err := reader.ReadMessage(ctx)
	if err != nil {
		k.logger.Errorf("failed to read message from Kafka topic %s: %v", topic, err)
		return pubsub.Message{}, err
	}

	return pubsub.Message{
		Value: msgValue.Value,
		Topic: topic,
	}, nil
}

func (k *kafkaClient) Close() error {
	return k.writer.Close()
}
