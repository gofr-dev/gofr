package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"gofr.dev/pkg/gofr/datasource/pubsub"
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
	if len(conf.Brokers) == 0 {
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
func (k *kafkaClient) retryConnect(ctx context.Context) {
	for {
		time.Sleep(defaultRetryTimeout)

		err := k.initialize(ctx)
		if err != nil {
			var brokers any

			if len(k.config.Brokers) > 1 {
				brokers = k.config.Brokers
			} else {
				brokers = k.config.Brokers[0]
			}

			k.logger.Errorf("could not connect to Kafka at '%v', error: %v", brokers, err)

			continue
		}

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

// connectToBrokers connects to Kafka brokers with context support.
func connectToBrokers(ctx context.Context, brokers []string, dialer *kafka.Dialer, logger pubsub.Logger) ([]Connection, error) {
	conns := make([]Connection, 0)

	if len(brokers) == 0 {
		return nil, errBrokerNotProvided
	}

	for _, broker := range brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err != nil {
			logger.Errorf("failed to connect to broker %s: %v", broker, err)
			continue
		}

		conns = append(conns, conn)
	}

	if len(conns) == 0 {
		return nil, errFailedToConnectBrokers
	}

	return conns, nil
}

func createKafkaWriter(conf *Config, dialer *kafka.Dialer, logger pubsub.Logger) Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:      conf.Brokers,
		Dialer:       dialer,
		BatchSize:    conf.BatchSize,
		BatchBytes:   conf.BatchBytes,
		BatchTimeout: time.Duration(conf.BatchTimeout),
		Logger:       kafka.LoggerFunc(logger.Debugf),
	})
}
