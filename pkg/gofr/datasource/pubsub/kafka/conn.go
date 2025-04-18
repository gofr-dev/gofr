package kafka

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/segmentio/kafka-go"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

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
