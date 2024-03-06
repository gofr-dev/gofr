package kafka

import (
	"context"
	"errors"
	"sync"
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
	conn   Connection

	writer Writer
	reader map[string]Reader

	mu *sync.RWMutex

	logger  pubsub.Logger
	config  Config
	metrics Metrics
}

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger, metrics Metrics) *kafkaClient {
	err := validateConfigs(conf)
	if err != nil {
		logger.Errorf("could not initialize kafka, err : %v", err)

		return nil
	}

	conn, err := kafka.Dial("tcp", conf.Broker)
	if err != nil {
		logger.Errorf("Failed to connect to KAFKA at %v", conf.Broker)
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{conf.Broker},
		Dialer:  dialer,
	})

	reader := make(map[string]Reader)

	return &kafkaClient{
		config:  conf,
		dialer:  dialer,
		reader:  reader,
		conn:    conn,
		logger:  logger,
		writer:  writer,
		mu:      &sync.RWMutex{},
		metrics: metrics,
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
	k.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

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

	k.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

func (k *kafkaClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	k.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic)

	var reader Reader
	// Lock the reader map to ensure only one subscriber access the reader at a time
	k.mu.Lock()

	if k.reader[topic] == nil {
		k.reader[topic] = k.getNewReader(topic)
	}

	// Release the lock on the reader map after update
	k.mu.Unlock()

	// Read a single message from the topic
	reader = k.reader[topic]
	msg, err := reader.ReadMessage(ctx)

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

	k.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic)

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

func (k *kafkaClient) getNewReader(topic string) Reader {
	reader := kafka.NewReader(kafka.ReaderConfig{
		GroupID:     k.config.ConsumerGroupID,
		Brokers:     []string{k.config.Broker},
		Topic:       topic,
		MinBytes:    10e3,
		MaxBytes:    10e6,
		Dialer:      k.dialer,
		StartOffset: int64(k.config.OffSet),
	})

	return reader
}

func (k *kafkaClient) DeleteTopic(_ context.Context, name string) error {
	return k.conn.DeleteTopics(name)
}

func (k *kafkaClient) Controller() (broker kafka.Broker, err error) {
	return k.conn.Controller()
}

func (k *kafkaClient) CreateTopic(_ context.Context, name string) error {
	topics := kafka.TopicConfig{Topic: name, NumPartitions: 1, ReplicationFactor: 1}

	err := k.conn.CreateTopics(topics)
	if err != nil {
		return err
	}

	return nil
}
