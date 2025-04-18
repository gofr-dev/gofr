// Package kafka provides a client for interacting with Apache Kafka message queues.This package facilitates interaction
// with Apache Kafka, allowing publishing and subscribing to topics, managing consumer groups, and handling messages.
package kafka

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var (
	ErrConsumerGroupNotProvided    = errors.New("consumer group id not provided")
	errBrokerNotProvided           = errors.New("kafka broker address not provided")
	errPublisherNotConfigured      = errors.New("can't publish message. Publisher not configured or topic is empty")
	errBatchSize                   = errors.New("KAFKA_BATCH_SIZE must be greater than 0")
	errBatchBytes                  = errors.New("KAFKA_BATCH_BYTES must be greater than 0")
	errBatchTimeout                = errors.New("KAFKA_BATCH_TIMEOUT must be greater than 0")
	errClientNotConnected          = errors.New("kafka client not connected")
	errUnsupportedSASLMechanism    = errors.New("unsupported SASL mechanism")
	errSASLCredentialsMissing      = errors.New("SASL credentials missing")
	errUnsupportedSecurityProtocol = errors.New("unsupported security protocol")
	errNoActiveConnections         = errors.New("no active connections to brokers")
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
	MessageMultipleBrokers = "MULTIPLE_BROKERS"
)

type Config struct {
	Broker           []string
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

//nolint:revive // New allow returning unexported types as intended.
func New(conf *Config, logger pubsub.Logger, metrics Metrics) *kafkaClient {
	err := validateConfigs(conf)
	if err != nil {
		logger.Errorf("could not initialize kafka, error: %v", err)

		return nil
	}

	var brokers any

	if len(conf.Broker) > 1 {
		brokers = conf.Broker
	} else {
		brokers = conf.Broker[0]
	}

	logger.Debugf("connecting to Kafka brokers: '%v'", brokers)

	dialer, conn, writer, reader, err := initializeKafkaClient(conf, logger)
	if err != nil {
		logger.Errorf("failed to connect to kafka at %v, error: %v", conf.Broker, err)

		client := &kafkaClient{
			logger:  logger,
			config:  *conf,
			metrics: metrics,
			mu:      &sync.RWMutex{},
		}

		go retryConnect(client, conf, logger)

		return client
	}

	return &kafkaClient{
		config:  *conf,
		dialer:  dialer,
		reader:  reader,
		conn:    conn,
		logger:  logger,
		writer:  writer,
		mu:      &sync.RWMutex{},
		metrics: metrics,
	}
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

	if len(k.config.Broker) > 1 {
		hostName = MessageMultipleBrokers
	} else {
		hostName = k.config.Broker[0]
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

	if len(k.config.Broker) > 1 {
		hostName = "multiple brokers"
	} else {
		hostName = k.config.Broker[0]
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

//nolint:unused // We need this wrap around for testing purposes.
type Conn struct {
	conns []*kafka.Conn
}

func initializeKafkaClient(conf *Config, logger pubsub.Logger) (*kafka.Dialer, *multiConn, Writer, map[string]Reader, error) {
	dialer, err := setupDialer(conf)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	conns, err := connectToBrokers(conf.Broker, dialer, logger)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	multi := &multiConn{
		conns:  conns,
		dialer: dialer,
	}

	writer := createKafkaWriter(conf, dialer, logger)
	reader := make(map[string]Reader)

	logger.Logf("connected to %d Kafka brokers", len(conns))

	return dialer, multi, writer, reader, nil
}

func (k *kafkaClient) getNewReader(topic string) Reader {
	reader := kafka.NewReader(kafka.ReaderConfig{
		GroupID:     k.config.ConsumerGroupID,
		Brokers:     k.config.Broker,
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

type multiConn struct {
	conns  []Connection
	dialer *kafka.Dialer
	mu     sync.RWMutex
}

func (m *multiConn) Controller() (kafka.Broker, error) {
	if len(m.conns) == 0 {
		return kafka.Broker{}, errNoActiveConnections
	}

	// Try all connections until we find one that works
	for _, conn := range m.conns {
		if conn == nil {
			continue
		}

		controller, err := conn.Controller()
		if err == nil {
			return controller, nil
		}
	}

	return kafka.Broker{}, errNoActiveConnections
}

func (m *multiConn) CreateTopics(topics ...kafka.TopicConfig) error {
	controller, err := m.Controller()
	if err != nil {
		return err
	}

	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))

	controllerResolvedAddr, err := net.ResolveTCPAddr("tcp", controllerAddr)
	if err != nil {
		return err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, conn := range m.conns {
		if conn == nil {
			continue
		}

		if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			if tcpAddr.IP.Equal(controllerResolvedAddr.IP) && tcpAddr.Port == controllerResolvedAddr.Port {
				return conn.CreateTopics(topics...)
			}
		}
	}

	// If not found, create a new connection
	conn, err := m.dialer.DialContext(context.Background(), "tcp", controllerAddr)
	if err != nil {
		return err
	}

	m.conns = append(m.conns, conn)

	return conn.CreateTopics(topics...)
}

func (m *multiConn) DeleteTopics(topics ...string) error {
	controller, err := m.Controller()
	if err != nil {
		return err
	}

	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))

	controllerResolvedAddr, err := net.ResolveTCPAddr("tcp", controllerAddr)
	if err != nil {
		return err
	}

	for _, conn := range m.conns {
		if conn == nil {
			continue
		}

		if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			// Match IP (after resolution) and Port
			if tcpAddr.IP.Equal(controllerResolvedAddr.IP) && tcpAddr.Port == controllerResolvedAddr.Port {
				return conn.DeleteTopics(topics...)
			}
		}
	}

	// If not found, create a new connection
	conn, err := m.dialer.DialContext(context.Background(), "tcp", controllerAddr)
	if err != nil {
		return err
	}

	m.conns = append(m.conns, conn)

	return conn.DeleteTopics(topics...)
}

func (m *multiConn) Close() error {
	var err error

	for _, conn := range m.conns {
		if conn != nil {
			err = errors.Join(err, conn.Close())
		}
	}

	return err
}
