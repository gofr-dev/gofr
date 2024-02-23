package kafka

import (
	"context"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var (
	errBrokerNotProvided        = errors.New("kafka broker address not provided")
	errConsumerGroupNotProvided = errors.New("consumer group id not provided")
	errPublisherNotConfigured   = errors.New("can't publish message. Publisher not configured or topic is empty")
)

type Config struct {
	Broker          string
	Partition       int
	ConsumerGroupID string
	OffSet          int
}

type kafkaClient struct {
	dialer *kafka.Dialer
	writer *kafka.Writer
	reader map[string]*kafka.Reader

	logger pubsub.Logger
	config Config
}

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger) *kafkaClient {
	err := validateConfigs(conf)
	if err != nil {
		logger.Errorf("could not initialize kafka, err : %v", err)

		return nil
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{conf.Broker},
		Dialer:  dialer,
	})

	reader := make(map[string]*kafka.Reader)

	return &kafkaClient{
		config: conf,
		dialer: dialer,
		reader: reader,
		logger: logger,
		writer: writer,
	}
}

func validateConfigs(conf Config) error {
	if conf.Broker == "" {
		return errBrokerNotProvided
	}

	if conf.ConsumerGroupID == "" {
		return errConsumerGroupNotProvided
	}

	return nil
}

func (k *kafkaClient) Publish(ctx context.Context, topic string, message []byte) error {
	if k.writer == nil || topic == "" {
		return errPublisherNotConfigured
	}

	err := k.writer.WriteMessages(ctx,
		kafka.Message{
			Topic: topic,
			Value: message,
			Time:  time.Now(),
		},
	)

	if err != nil {
		k.logger.Error("failed to publish message to kafka broker")
		return err
	}

	k.logger.Debugf("published kafka message %v on topic %v", string(message), topic)

	return nil
}

func (k *kafkaClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if k.reader[topic] == nil {
		reader := kafka.NewReader(kafka.ReaderConfig{
			GroupID:     k.config.ConsumerGroupID,
			Brokers:     []string{k.config.Broker},
			Topic:       topic,
			MinBytes:    10e3,
			MaxBytes:    10e6,
			Dialer:      k.dialer,
			StartOffset: int64(k.config.OffSet),
		})

		k.reader[topic] = reader
	}

	// Read a single message from the topic
	msg, err := k.reader[topic].ReadMessage(ctx)
	if err != nil {
		k.logger.Errorf("failed to read message from Kafka topic %s: %v", topic, err)

		return nil, err
	}

	m := &pubsub.Message{
		Value: msg.Value,
		Topic: topic,

		Committer: newKafkaMessage(&msg, k.reader[topic], k.logger),
	}

	k.logger.Debugf("received kafka message %v on topic %v", string(msg.Value), msg.Topic)

	return m, err
}

func (k *kafkaClient) Close() error {
	err := k.writer.Close()
	if err != nil {
		k.logger.Errorf("failed to close Kafka writer: %v", err)

		return err
	}

	return nil
}
