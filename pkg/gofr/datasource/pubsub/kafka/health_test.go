package kafka

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gofr.dev/pkg/gofr/datasource"
)

// MockConn simulates a Kafka connection
type MockConn struct {
	mock.Mock
	addr      string
	isHealthy bool
	isControl bool
}

func (m *MockConn) RemoteAddr() net.Addr {
	return mockAddr{m.addr}
}

func (m *MockConn) ReadPartitions(topics ...string) ([]kafka.Partition, error) {
	if m.isHealthy {
		return []kafka.Partition{{}}, nil
	}
	return nil, errors.New("unreachable")
}

func (m *MockConn) Controller() (kafka.Broker, error) {
	if m.isControl {
		host, _, _ := net.SplitHostPort(m.addr)
		port := 9092
		return kafka.Broker{Host: host, Port: port}, nil
	}
	return kafka.Broker{}, errors.New("not a controller")
}

// Add minimal required method stubs
func (m *MockConn) Close() error                            { return nil }
func (m *MockConn) CreateTopics(...kafka.TopicConfig) error { return nil }
func (m *MockConn) DeleteTopics(...string) error            { return nil }

type mockAddr struct{ addr string }

func (m mockAddr) Network() string { return "tcp" }
func (m mockAddr) String() string  { return m.addr }

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

func (m *mockWriter) Stats() kafka.WriterStats {
	return kafka.WriterStats{
		Dials:    1,
		Writes:   1,
		Messages: 1,
		Bytes:    1024,
		Errors:   0,
	}
}

func (m *mockWriter) WriteMessages(ctx context.Context, msg ...kafka.Message) error { return nil }
func (m *mockWriter) Close() error                                                  { return nil }

type mockReader struct{}

func (m *mockReader) Stats() any                      { return map[string]string{"reader": "value"} }
func (m *mockReader) FetchMessage(_ any) (any, error) { return nil, nil }
func (m *mockReader) Close() error                    { return nil }

type mockLogger struct{}

func (m *mockLogger) Errorf(format string, args ...any) {}
func (m *mockLogger) Debugf(format string, args ...any) {}
func (m *mockLogger) Logf(format string, args ...any)   {}
func (m *mockLogger) Log(args ...any)                   {}
func (m *mockLogger) Error(args ...any)                 {}
func (m *mockLogger) Debug(args ...any)                 {}
