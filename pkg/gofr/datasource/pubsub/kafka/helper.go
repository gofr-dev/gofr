package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	k.connMu.RLock()
	defer k.connMu.RUnlock()

	return k.isConnectedLocked()
}

// isConnectedLocked probes the admin connection. The caller must already hold
// connMu (read or write); used by ensureConnected to avoid re-acquiring the
// lock between the unlocked fast path and the locked double-check.
func (k *kafkaClient) isConnectedLocked() bool {
	if k.conn == nil {
		return false
	}

	_, err := k.conn.Controller()

	return err == nil
}

// ensureConnected verifies the admin connection is alive and, if not, makes a
// single attempt to re-dial the brokers. Kafka brokers silently drop idle TCP
// connections after their configured idle timeout (default
// connections.max.idle.ms = 10 min), so without a runtime reconnect path the
// admin conn pool dialed once in initialize() stays stale forever and every
// Subscribe/Query call returns errClientNotConnected even though the cluster
// is healthy.
//
// The reconnect is serialized on connMu — a dedicated lock that does not
// block subscribers holding the reader-map lock (k.mu). Only k.conn and
// k.dialer are refreshed; in-flight writers and per-topic readers manage
// their own connections via segmentio/kafka-go.
func (k *kafkaClient) ensureConnected(ctx context.Context) bool {
	if k.isConnected() {
		return true
	}

	k.connMu.Lock()
	defer k.connMu.Unlock()

	// Re-check inside the write lock — another goroutine may have
	// already reconnected while we were waiting on the mutex.
	if k.isConnectedLocked() {
		return true
	}

	if err := k.reconnectAdminLocked(ctx); err != nil {
		// Throttle error-level logs: in a high-QPS service every
		// failed Subscribe/Query would otherwise log Errorf, swamping
		// the log pipeline while the cluster is unreachable. Log at
		// error once per reconnectErrLogInterval; in between, log at
		// debug so the failure is still observable when needed.
		now := time.Now()
		if now.After(k.reconnectErrLogAt) {
			k.logger.Errorf("kafka admin reconnect failed: %v", err)
			k.reconnectErrLogAt = now.Add(reconnectErrLogInterval)
		} else {
			k.logger.Debugf("kafka admin reconnect failed: %v", err)
		}

		return false
	}

	// Reset the throttle so the next outage starts with an Errorf again.
	k.reconnectErrLogAt = time.Time{}
	k.logger.Log("reconnected to kafka after stale admin connection")

	return true
}

// reconnectAdminLocked re-dials the broker-facing admin connections used for
// Controller/CreateTopic/DeleteTopic and the isConnected health probe. The
// caller MUST hold connMu for writing. It deliberately does not touch the
// writer or reader map — those keep their own connections.
func (k *kafkaClient) reconnectAdminLocked(ctx context.Context) error {
	dialer := k.dialer
	if dialer == nil {
		d, err := setupDialer(&k.config)
		if err != nil {
			return err
		}

		dialer = d
	}

	conns, err := connectToBrokers(ctx, k.config.Brokers, dialer, k.logger)
	if err != nil {
		return err
	}

	old := k.conn
	k.conn = &multiConn{
		conns:  conns,
		dialer: dialer,
	}
	k.dialer = dialer

	// Safe to close while holding the write lock: no other goroutine can
	// be using the old multiConn because admin entry points all take
	// connMu.RLock before touching k.conn.
	if old != nil {
		_ = old.Close()
	}

	return nil
}

func setupDialer(conf *Config) (*kafka.Dialer, error) {
	dialer := &kafka.Dialer{
		Timeout:   defaultRetryTimeout,
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
//
// Exposed as a var so tests can stub the network dial in reconnectAdminLocked
// and initialize without spinning up a real broker. Production callers must
// not reassign this.
//
//nolint:gochecknoglobals // Test seam — see doc above. Reassigned only from tests via t.Cleanup.
var connectToBrokers = func(ctx context.Context, brokers []string, dialer *kafka.Dialer, logger pubsub.Logger) ([]Connection, error) {
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

func (*kafkaClient) parseQueryArgs(args ...any) (offSet int64, limit int) {
	var offset int64

	limit = 10

	if len(args) > 0 {
		if val, ok := args[0].(int64); ok {
			offset = val
		}
	}

	if len(args) > 1 {
		if val, ok := args[1].(int); ok {
			limit = val
		}
	}

	return offset, limit
}

func (k *kafkaClient) createReader(topic string, offset int64) (*kafka.Reader, error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.config.Brokers,
		Topic:       topic,
		Partition:   k.config.Partition,
		MinBytes:    1,
		MaxBytes:    defaultMaxBytes,
		StartOffset: kafka.FirstOffset,
	})

	if err := reader.SetOffset(offset); err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to set offset: %w", err)
	}

	return reader, nil
}

func (*kafkaClient) getReadContext(ctx context.Context) context.Context {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		readCtx, cancel := context.WithTimeout(ctx, defaultReadTimeout)
		_ = cancel // We can't defer here, but timeout will handle cleanup

		return readCtx
	}

	return ctx
}

func (k *kafkaClient) readMessages(ctx context.Context, reader *kafka.Reader, limit int) ([]byte, error) {
	var result []byte

	for i := 0; i < limit; i++ {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if k.isExpectedError(err) {
				break
			}

			return nil, fmt.Errorf("failed to read message: %w", err)
		}

		if len(result) > 0 {
			result = append(result, '\n')
		}

		result = append(result, msg.Value...)
	}

	return result, nil
}

func (*kafkaClient) isExpectedError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF)
}
