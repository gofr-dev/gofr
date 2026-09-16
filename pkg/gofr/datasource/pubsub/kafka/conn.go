package kafka

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/segmentio/kafka-go"
)

//nolint:unused // We need this wrap around for testing purposes.
type Conn struct {
	conns []*kafka.Conn
}

// initialize creates and configures all Kafka client components.
//
// initialize is called once from New() before the client is returned, and
// again from retryConnect() in a goroutine if the first attempt fails.
// Because retryConnect runs concurrently with user-facing calls, the
// k.conn / k.dialer writes must be serialized against the readers that
// take connMu (Health, Controller, ensureConnected, getNewReader, ...).
func (k *kafkaClient) initialize(ctx context.Context) error {
	dialer, err := setupDialer(&k.config)
	if err != nil {
		return err
	}

	conns, err := connectToBrokers(ctx, k.config.Brokers, dialer, k.logger)
	if err != nil {
		return err
	}

	multi := &multiConn{
		conns:  conns,
		dialer: dialer,
	}

	writer := createKafkaWriter(&k.config, dialer, k.logger)
	reader := make(map[string]Reader)

	k.logger.Logf("connected to %d Kafka brokers", len(conns))

	k.connMu.Lock()

	// Close only signals the retry goroutine; it cannot interrupt a dial
	// already in flight. Re-check under the same lock Close takes, so a
	// retryConnect that got past its own check cannot revive a closed client
	// and leak this conn pool and writer for the life of the process.
	// A nil k.closed (a client built without New) never fires, so the
	// default arm is the normal path.
	select {
	case <-k.closed:
		k.connMu.Unlock()

		return errors.Join(errClientClosed, multi.Close(), writer.Close())
	default:
	}

	k.dialer = dialer
	k.conn = multi

	// writer is published under connMu alongside conn: retryConnect calls
	// initialize from its own goroutine, so this is a pointer swap that
	// Publish, Close and Health can all be reading concurrently.
	k.writer = writer

	k.connMu.Unlock()

	// retryConnect calls initialize from its own goroutine, so this swap can
	// land while a Subscribe is reading or growing the map. Take k.mu for it,
	// the same lock Subscribe uses, and keep any readers a concurrent
	// Subscribe already created rather than dropping them on the floor —
	// their committers are already handed out to callers.
	k.mu.Lock()

	if k.reader == nil {
		k.reader = reader
	}

	k.mu.Unlock()

	return nil
}

func (k *kafkaClient) getNewReader(topic string) Reader {
	// Snapshot the dialer under connMu — reconnectAdminLocked may swap it
	// concurrently. Once handed to kafka.NewReader, the reader keeps its
	// own reference; later reconnects do not affect existing readers.
	k.connMu.RLock()
	dialer := k.dialer
	k.connMu.RUnlock()

	reader := kafka.NewReader(kafka.ReaderConfig{
		GroupID:     k.config.ConsumerGroupID,
		Brokers:     k.config.Brokers,
		Topic:       topic,
		MinBytes:    defaultMinBytes,
		MaxBytes:    defaultMaxBytes,
		Dialer:      dialer,
		StartOffset: int64(k.config.OffSet),
	})

	return reader
}

func (k *kafkaClient) DeleteTopic(_ context.Context, name string) error {
	k.connMu.RLock()
	defer k.connMu.RUnlock()

	if k.conn == nil {
		return errClientNotConnected
	}

	return k.conn.DeleteTopics(name)
}

func (k *kafkaClient) Controller() (broker kafka.Broker, err error) {
	k.connMu.RLock()
	defer k.connMu.RUnlock()

	if k.conn == nil {
		return kafka.Broker{}, errClientNotConnected
	}

	return k.conn.Controller()
}

func (k *kafkaClient) CreateTopic(_ context.Context, name string) error {
	k.connMu.RLock()
	defer k.connMu.RUnlock()

	if k.conn == nil {
		return errClientNotConnected
	}

	topics := kafka.TopicConfig{Topic: name, NumPartitions: 1, ReplicationFactor: 1}

	return k.conn.CreateTopics(topics)
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
