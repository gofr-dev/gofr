package kafka

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"time"
)

func validateConfigs(conf *Config) error {
	if err := validateRequiredFields(conf); err != nil {
		return err
	}

	setDefaultSecurityProtocol(conf)

	if err := validateSASLConfigs(conf); err != nil {
		return err
	}

	if err := validateTLSConfigs(conf); err != nil {
		return err
	}

	if err := validateSecurityProtocol(conf); err != nil {
		return err
	}

	return nil
}

func validateRequiredFields(conf *Config) error {
	if len(conf.Broker) == 0 {
		return errBrokerNotProvided
	}

	if conf.BatchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0: %w", errBatchSize)
	}

	if conf.BatchBytes <= 0 {
		return fmt.Errorf("batch bytes must be greater than 0: %w", errBatchBytes)
	}

	if conf.BatchTimeout <= 0 {
		return fmt.Errorf("batch timeout must be greater than 0: %w", errBatchTimeout)
	}

	return nil
}

// retryConnect handles the retry mechanism for connecting to the Kafka broker.
func retryConnect(client *kafkaClient, conf *Config, logger pubsub.Logger) {
	for {
		time.Sleep(defaultRetryTimeout)

		dialer, conn, writer, reader, err := initializeKafkaClient(conf, logger)
		if err != nil {
			var brokers any

			if len(conf.Broker) > 1 {
				brokers = conf.Broker
			} else {
				brokers = conf.Broker[0]
			}

			logger.Errorf("could not connect to Kafka at '%v', error: %v", brokers, err)

			continue
		}

		client.conn = conn
		client.dialer = dialer
		client.writer = writer
		client.reader = reader

		return
	}
}

func (k *kafkaClient) isConnected() bool {
	if k.conn == nil {
		return false
	}

	_, err := k.conn.Controller()

	return err == nil
}

func setupDialer(conf *Config) (*kafka.Dialer, error) {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	if conf.SecurityProtocol == protocolSASL || conf.SecurityProtocol == protocolSASLSSL {
		mechanism, err := getSASLMechanism(conf.SASLMechanism, conf.SASLUser, conf.SASLPassword)
		if err != nil {
			return nil, err
		}

		dialer.SASLMechanism = mechanism
	}

	if conf.SecurityProtocol == "SSL" || conf.SecurityProtocol == "SASL_SSL" {
		tlsConfig, err := createTLSConfig(&conf.TLS)
		if err != nil {
			return nil, err
		}

		dialer.TLS = tlsConfig
	}

	return dialer, nil
}

func connectToBrokers(brokers []string, dialer *kafka.Dialer, logger pubsub.Logger) ([]Connection, error) {
	if len(brokers) == 0 {
		return nil, errBrokerNotProvided
	}

	conns := make([]Connection, 0)

	for _, broker := range brokers {
		conn, err := dialer.DialContext(context.Background(), "tcp", broker)
		if err != nil {
			logger.Errorf("failed to connect to broker %s: %v", broker, err)
			continue
		}

		conns = append(conns, conn)
	}

	if len(conns) == 0 {
		return nil, errNoActiveConnections
	}

	return conns, nil
}

func createKafkaWriter(conf *Config, dialer *kafka.Dialer, logger pubsub.Logger) Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:      conf.Broker,
		Dialer:       dialer,
		BatchSize:    conf.BatchSize,
		BatchBytes:   conf.BatchBytes,
		BatchTimeout: time.Duration(conf.BatchTimeout),
		Logger:       kafka.LoggerFunc(logger.Debugf),
	})
}
