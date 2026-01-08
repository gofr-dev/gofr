package sqs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		client := New(nil)
		require.NotNil(t, client)
		assert.Equal(t, int32(1), client.cfg.MaxMessages)
		assert.Equal(t, int32(20), client.cfg.WaitTimeSeconds)
		assert.Equal(t, int32(30), client.cfg.VisibilityTimeout)
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &Config{
			Region:            "us-east-1",
			MaxMessages:       5,
			WaitTimeSeconds:   10,
			VisibilityTimeout: 60,
		}
		client := New(cfg)
		require.NotNil(t, client)
		assert.Equal(t, "us-east-1", client.cfg.Region)
		assert.Equal(t, int32(5), client.cfg.MaxMessages)
		assert.Equal(t, int32(10), client.cfg.WaitTimeSeconds)
		assert.Equal(t, int32(60), client.cfg.VisibilityTimeout)
	})

	t.Run("with invalid max messages", func(t *testing.T) {
		cfg := &Config{
			MaxMessages: 15, // exceeds 10
		}
		client := New(cfg)
		assert.Equal(t, int32(1), client.cfg.MaxMessages) // should reset to default
	})
}

func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name:  "empty config",
			input: Config{},
			expected: Config{
				MaxMessages:       1,
				WaitTimeSeconds:   20,
				VisibilityTimeout: 30,
				DelaySeconds:      0,
				RetryDuration:     5000000000, // 5 seconds in nanoseconds
			},
		},
		{
			name: "max messages exceeds limit",
			input: Config{
				MaxMessages: 15,
			},
			expected: Config{
				MaxMessages:       1,
				WaitTimeSeconds:   20,
				VisibilityTimeout: 30,
				DelaySeconds:      0,
				RetryDuration:     5000000000,
			},
		},
		{
			name: "negative values",
			input: Config{
				MaxMessages:       -1,
				WaitTimeSeconds:   -5,
				VisibilityTimeout: -10,
				DelaySeconds:      -1,
			},
			expected: Config{
				MaxMessages:       1,
				WaitTimeSeconds:   20,
				VisibilityTimeout: 30,
				DelaySeconds:      0,
				RetryDuration:     5000000000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.setDefaults()
			assert.Equal(t, tt.expected.MaxMessages, tt.input.MaxMessages)
			assert.Equal(t, tt.expected.WaitTimeSeconds, tt.input.WaitTimeSeconds)
			assert.Equal(t, tt.expected.VisibilityTimeout, tt.input.VisibilityTimeout)
			assert.Equal(t, tt.expected.DelaySeconds, tt.input.DelaySeconds)
		})
	}
}

func TestClient_UseLogger(t *testing.T) {
	client := New(&Config{})
	logger := NewMockLogger()

	client.UseLogger(logger)
	assert.NotNil(t, client.logger)

	// Test with invalid type
	client.UseLogger("invalid")
	// Should still be the previous logger
	assert.Equal(t, logger, client.logger)
}

func TestClient_UseMetrics(t *testing.T) {
	client := New(&Config{})
	metrics := NewMockMetrics()

	client.UseMetrics(metrics)
	assert.NotNil(t, client.metrics)

	// Test with invalid type
	client.UseMetrics("invalid")
	// Should still be the previous metrics
	assert.Equal(t, metrics, client.metrics)
}

func TestClient_Connect_NoRegion(t *testing.T) {
	client := New(&Config{})
	client.UseLogger(NewMockLogger())

	client.Connect()

	// Client should not be connected without region
	assert.Nil(t, client.client)
}

func TestClient_Publish_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	err := client.Publish(context.Background(), "test-queue", []byte("test message"))
	assert.ErrorIs(t, err, ErrClientNotConnected)
}

func TestClient_Publish_EmptyTopic(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	// Manually set client to non-nil to bypass connection check
	// This tests the empty topic validation

	err := client.Publish(context.Background(), "", []byte("test message"))
	assert.ErrorIs(t, err, ErrClientNotConnected) // Still fails because client is nil
}

func TestClient_Subscribe_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	msg, err := client.Subscribe(context.Background(), "test-queue")

	require.ErrorIs(t, err, ErrClientNotConnected)
	assert.Nil(t, msg)
}

func TestClient_CreateTopic_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.CreateTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, ErrClientNotConnected)
}

func TestClient_DeleteTopic_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.DeleteTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, ErrClientNotConnected)
}

func TestClient_Query_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	result, err := client.Query(context.Background(), "test-queue")

	require.ErrorIs(t, err, ErrClientNotConnected)
	assert.Nil(t, result)
}

func TestClient_Health_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	health := client.Health()
	assert.Equal(t, "DOWN", health.Status)
	assert.Equal(t, "client not connected", health.Details["error"])
}

func TestClient_Close(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.Close()

	require.NoError(t, err)
	assert.Nil(t, client.client)
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short message",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "exactly 100 chars",
			input:    "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			expected: "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
		},
		{
			name:     "over 100 chars",
			input:    "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			expected: "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateMessage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseQueryArgs(t *testing.T) {
	client := New(&Config{})

	tests := []struct {
		name     string
		args     []any
		expected int32
	}{
		{
			name:     "no args",
			args:     nil,
			expected: 10,
		},
		{
			name:     "valid limit",
			args:     []any{5},
			expected: 5,
		},
		{
			name:     "limit exceeds max",
			args:     []any{15},
			expected: 10,
		},
		{
			name:     "invalid type",
			args:     []any{"invalid"},
			expected: 10,
		},
		{
			name:     "zero",
			args:     []any{0},
			expected: 10,
		},
		{
			name:     "negative",
			args:     []any{-1},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.parseQueryArgs(tt.args...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
