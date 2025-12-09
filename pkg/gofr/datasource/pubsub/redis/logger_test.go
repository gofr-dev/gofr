package redis

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrettyPrint(t *testing.T) { //nolint:funlen // Test function with many test cases
	tests := []struct {
		name     string
		log      *Log
		validate func(t *testing.T, output string)
	}{
		{
			name: "pretty print with all fields",
			log: &Log{
				Mode:          "PUB",
				CorrelationID: "12345-67890",
				MessageValue:  "test message",
				Topic:         "test-topic",
				Host:          "localhost:6379",
				PubSubBackend: "REDIS",
				Time:          1000,
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "12345-67890")
				assert.Contains(t, output, "REDIS")
				assert.Contains(t, output, "1000")
				assert.Contains(t, output, "PUB")
				assert.Contains(t, output, "test-topic")
				assert.Contains(t, output, "test message")
			},
		},
		{
			name: "pretty print with SUB mode",
			log: &Log{
				Mode:          "SUB",
				CorrelationID: "abc-123",
				MessageValue:  "subscribed message",
				Topic:         "sub-topic",
				Host:          "redis.example.com:6379",
				PubSubBackend: "REDIS",
				Time:          500,
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "abc-123")
				assert.Contains(t, output, "SUB")
				assert.Contains(t, output, "sub-topic")
				assert.Contains(t, output, "subscribed message")
			},
		},
		{
			name: "pretty print with zero time",
			log: &Log{
				Mode:          "PUB",
				CorrelationID: "zero-time",
				MessageValue:  "message",
				Topic:         "topic",
				Host:          "localhost:6379",
				PubSubBackend: "REDIS",
				Time:          0,
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "zero-time")
				assert.Contains(t, output, "0")
			},
		},
		{
			name: "pretty print with empty fields",
			log: &Log{
				Mode:          "",
				CorrelationID: "",
				MessageValue:  "",
				Topic:         "",
				Host:          "",
				PubSubBackend: "",
				Time:          0,
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				// Should not panic, just print empty values
				assert.NotEmpty(t, output) // Should still produce output
			},
		},
		{
			name: "pretty print with long correlation ID",
			log: &Log{
				Mode:          "PUB",
				CorrelationID: "very-long-correlation-id-that-exceeds-normal-length",
				MessageValue:  "test",
				Topic:         "topic",
				Host:          "localhost:6379",
				PubSubBackend: "REDIS",
				Time:          1234,
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "very-long-correlation-id-that-exceeds-normal-length")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.log.PrettyPrint(&buf)
			output := buf.String()
			tt.validate(t, output)
		})
	}
}

func TestLoggerAdapter(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *loggerAdapter
		method   string
		args     []any
		validate func(t *testing.T, adapter *loggerAdapter)
	}{
		{
			name: "Debug method",
			setup: func(_ *testing.T) *loggerAdapter {
				return &loggerAdapter{
					Logger: &mockPubSubLogger{},
				}
			},
			method: "Debug",
			args:   []any{"test", "message"},
			validate: func(_ *testing.T, adapter *loggerAdapter) {
				// Should not panic
				adapter.Debug("test", "message")
			},
		},
		{
			name: "Log method",
			setup: func(_ *testing.T) *loggerAdapter {
				return &loggerAdapter{
					Logger: &mockPubSubLogger{},
				}
			},
			method: "Log",
			args:   []any{"log", "message"},
			validate: func(_ *testing.T, adapter *loggerAdapter) {
				// Should not panic
				adapter.Log("log", "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := tt.setup(t)
			tt.validate(t, adapter)
		})
	}
}

// mockPubSubLogger is a mock implementation of pubsub.Logger.
type mockPubSubLogger struct{}

func (*mockPubSubLogger) Debugf(_ string, _ ...any) {}
func (*mockPubSubLogger) Debug(_ ...any)            {}
func (*mockPubSubLogger) Logf(_ string, _ ...any)   {}
func (*mockPubSubLogger) Log(_ ...any)              {}
func (*mockPubSubLogger) Errorf(_ string, _ ...any) {}
func (*mockPubSubLogger) Error(_ ...any)            {}
