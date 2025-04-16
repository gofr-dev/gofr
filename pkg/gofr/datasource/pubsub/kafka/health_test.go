package kafka

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gofr.dev/pkg/gofr/datasource"
)

var (
	errNotController = errors.New("not a controller")
	errUnreachable   = errors.New("unreachable")
)

// MockConn simulates a Kafka connection.
type MockConn struct {
	mock.Mock
	addr      string
	isHealthy bool
	isControl bool
}

func (m *MockConn) RemoteAddr() net.Addr {
	return mockAddr{m.addr}
}

func (m *MockConn) ReadPartitions(...string) ([]kafka.Partition, error) {
	if m.isHealthy {
		return []kafka.Partition{{}}, nil
	}
	return nil, errUnreachable
}

func (m *MockConn) Controller() (kafka.Broker, error) {
	if m.isControl {
		host, _, _ := net.SplitHostPort(m.addr)

		port := 9092

		return kafka.Broker{Host: host, Port: port}, nil
	}

	return kafka.Broker{}, errNotController
}

// Add minimal required method stubs.
func (*MockConn) Close() error                            { return nil }
func (*MockConn) CreateTopics(...kafka.TopicConfig) error { return nil }
func (*MockConn) DeleteTopics(...string) error            { return nil }

type mockAddr struct{ addr string }

func (mockAddr) Network() string  { return "tcp" }
func (m mockAddr) String() string { return m.addr }

func TestKafkaHealth_AllBrokersUp(t *testing.T) {
	client := &kafkaClient{
		conn: &multiConn{
			conns: []Connection{
				&MockConn{addr: "127.0.0.1:9092", isHealthy: true, isControl: true},
				&MockConn{addr: "127.0.0.2:9092", isHealthy: true, isControl: false},
			},
		},
		reader: make(map[string]Reader),
		writer: &mockWriter{},
		logger: &mockLogger{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Len(t, health.Details["brokers"], 2)
	assert.Contains(t, health.Details["brokers"], map[string]interface{}{
		"broker":       "127.0.0.1:9092",
		"status":       "UP",
		"isController": true,
		"error":        nil,
	})
}

func TestKafkaHealth_SomeBrokersUpSomeDown(t *testing.T) {
	client := &kafkaClient{
		conn: &multiConn{
			conns: []Connection{
				&MockConn{addr: "127.0.0.1:9092", isHealthy: true, isControl: false},
				&MockConn{addr: "127.0.0.2:9092", isHealthy: false},
				&MockConn{addr: "127.0.0.3:9092", isHealthy: true, isControl: true},
			},
		},
		reader: make(map[string]Reader),
		writer: &mockWriter{},
		logger: &mockLogger{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status) // Because at least one broker is down

	brokers := health.Details["brokers"].([]map[string]interface{})
	assert.Len(t, brokers, 3)

	statusMap := map[string]string{}

	for _, broker := range brokers {
		addr := broker["broker"].(string)
		status := broker["status"].(string)
		statusMap[addr] = status
	}

	assert.Equal(t, "UP", statusMap["127.0.0.1:9092"])
	assert.Equal(t, "DOWN", statusMap["127.0.0.2:9092"])
	assert.Equal(t, "UP", statusMap["127.0.0.3:9092"])
}

func TestKafkaHealth_AllBrokersDown(t *testing.T) {
	client := &kafkaClient{
		conn: &multiConn{
			conns: []Connection{
				&MockConn{addr: "127.0.0.1:9092", isHealthy: false},
			},
		},
		reader: make(map[string]Reader),
		writer: &mockWriter{},
		logger: &mockLogger{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Len(t, health.Details["brokers"], 1)

	brokerInfo := health.Details["brokers"].([]map[string]interface{})[0]

	assert.Equal(t, "DOWN", brokerInfo["status"])
	assert.NotNil(t, brokerInfo["error"])
}

func TestKafkaHealth_InvalidConnType(t *testing.T) {
	client := &kafkaClient{
		conn:   nil,
		reader: make(map[string]Reader),
		writer: &mockWriter{},
		logger: &mockLogger{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "invalid connection type", health.Details["error"])
}

// --- Mock implementations for Writer/Reader/Logger/Stats

type mockWriter struct{}

func (*mockWriter) Stats() kafka.WriterStats {
	return kafka.WriterStats{
		Dials:    1,
		Writes:   1,
		Messages: 1,
		Bytes:    1024,
		Errors:   0,
	}
}

func (*mockWriter) WriteMessages(context.Context, ...kafka.Message) error { return nil }
func (*mockWriter) Close() error                                          { return nil }

type mockReader struct{}

func (*mockReader) Stats() any                      { return map[string]string{"reader": "value"} }
func (*mockReader) FetchMessage(_ any) (any, error) { return nil, nil }
func (*mockReader) Close() error                    { return nil }

type mockLogger struct{}

func (*mockLogger) Errorf(string, ...any) {}
func (*mockLogger) Debugf(string, ...any) {}
func (*mockLogger) Logf(string, ...any)   {}
func (*mockLogger) Log(...any)            {}
func (*mockLogger) Error(...any)          {}
func (*mockLogger) Debug(...any)          {}
