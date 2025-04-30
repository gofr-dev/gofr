// Package kafka provides a client for interacting with Apache Kafka message queues.This package facilitates interaction
// with Apache Kafka, allowing publishing and subscribing to topics, managing consumer groups, and handling messages.
package kafka

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	DefaultBatchSize       = 100
	DefaultBatchBytes      = 1048576
	DefaultBatchTimeout    = 1000
	defaultRetryTimeout    = 10 * time.Second
	protocolPlainText      = "PLAINTEXT"
	protocolSASL           = "SASL_PLAINTEXT"
	protocolSSL            = "SSL"
	protocolSASLSSL        = "SASL_SSL"
	messageMultipleBrokers = "MULTIPLE_BROKERS"
	brokerStatusUp         = "UP"
)

type Config struct {
	Brokers          []string
	Partition        int
	ConsumerGroupID  string
	OffSet           int
	BatchSize        int
	BatchBytes       int
	BatchTimeout     int
	RetryTimeout     time.Duration
	SASLMechanism    string
	SASLUser         string
	SASLPassword     string
	SecurityProtocol string
	TLS              TLSConfig
}

type kafkaClient struct {
	dialer *kafka.Dialer
	conn   *multiConn

	writer Writer
	reader map[string]Reader

	mu *sync.RWMutex

	logger  pubsub.Logger
	config  Config
	metrics Metrics
}

func New(conf *Config, logger pubsub.Logger, metrics Metrics) *kafkaClient { //nolint:revive // New allows
	// returning unexported types as intended.
	err := validateConfigs(conf)
	if err != nil {
		logger.Errorf("could not initialize kafka, error: %v", err)

		return nil
	}

	if len(conf.Brokers) == 1 {
		logger.Debugf("connecting to Kafka broker: '%s'", conf.Brokers[0])
	} else {
		logger.Debugf("connecting to Kafka brokers: %v", conf.Brokers)
	}

	client := &kafkaClient{
		logger:  logger,
		config:  *conf,
		metrics: metrics,
		mu:      &sync.RWMutex{},
	}
	ctx := context.Background()

	err = client.initialize(ctx)

	if err != nil {
		logger.Errorf("failed to connect to kafka at %v, error: %v", conf.Brokers, err)

		go client.retryConnect(ctx)

		return client
	}

	return client
}

func (k *kafkaClient) Publish(ctx context.Context, topic string, message []byte) error {
	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "kafka-publish")
	defer span.End()

	k.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	if k.writer == nil || topic == "" {
		return errPublisherNotConfigured
	}

	start := time.Now()
	err := k.writer.WriteMessages(ctx,
		kafka.Message{
			Topic: topic,
			Value: message,
			Time:  time.Now(),
		},
	)
	end := time.Since(start)

	if err != nil {
		k.logger.Errorf("failed to publish message to kafka broker, error: %v", err)
		return err
	}

	var hostName string

	if len(k.config.Brokers) > 1 {
		hostName = messageMultipleBrokers
	} else {
		hostName = k.config.Brokers[0]
	}

	k.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         topic,
		Host:          hostName,
		PubSubBackend: "KAFKA",
		Time:          end.Microseconds(),
	})

	k.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

func (k *kafkaClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if !k.isConnected() {
		time.Sleep(defaultRetryTimeout)

		return nil, errClientNotConnected
	}

	if k.config.ConsumerGroupID == "" {
		k.logger.Error("cannot subscribe as consumer_id is not provided in configs")

		return &pubsub.Message{}, ErrConsumerGroupNotProvided
	}

	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "kafka-subscribe")
	defer span.End()

	k.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic, "consumer_group", k.config.ConsumerGroupID)

	var reader Reader
	// Lock the reader map to ensure only one subscriber access the reader at a time
	k.mu.Lock()

	if k.reader == nil {
		k.reader = make(map[string]Reader)
	}

	if k.reader[topic] == nil {
		k.reader[topic] = k.getNewReader(topic)
	}

	// Release the lock on the reader map after update
	k.mu.Unlock()

	start := time.Now()

	// Read a single message from the topic
	reader = k.reader[topic]
	msg, err := reader.FetchMessage(ctx)

	if err != nil {
		k.logger.Errorf("failed to read message from kafka topic %s: %v", topic, err)

		return nil, err
	}

	m := pubsub.NewMessage(ctx)
	m.Value = msg.Value
	m.Topic = topic
	m.Committer = newKafkaMessage(&msg, k.reader[topic], k.logger)

	end := time.Since(start)

	var hostName string

	if len(k.config.Brokers) > 1 {
		hostName = "multiple brokers"
	} else {
		hostName = k.config.Brokers[0]
	}

	k.logger.Debug(&pubsub.Log{
		Mode:          "SUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(msg.Value),
		Topic:         topic,
		Host:          hostName,
		PubSubBackend: "KAFKA",
		Time:          end.Microseconds(),
	})

	k.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic, "consumer_group", k.config.ConsumerGroupID)

	return m, err
}

func (k *kafkaClient) Close() (err error) {
	for _, r := range k.reader {
		err = errors.Join(err, r.Close())
	}

	if k.writer != nil {
		err = errors.Join(err, k.writer.Close())
	}

	if k.conn != nil {
		err = errors.Join(k.conn.Close())
	}

	return err
}
