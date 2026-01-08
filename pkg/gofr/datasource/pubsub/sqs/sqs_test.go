package sqs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

var (
	errWrappedContextCanceled  = errors.New("operation failed: context canceled")
	errWrappedDeadlineExceeded = errors.New("timeout: context deadline exceeded")
	errRequestCanceled         = errors.New("request was canceled")
	errGeneric                 = errors.New("some other error")
	errConnectionRefused       = errors.New("dial tcp: connection refused")
	errNoSuchHost              = errors.New("dial tcp: no such host")
	errNetworkUnreachable      = errors.New("dial tcp: network is unreachable")
	errMaxAttemptsExceeded     = errors.New("exceeded maximum number of attempts")
)

func TestNew(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		client := New(nil)
		require.NotNil(t, client)
		assert.NotNil(t, client.cfg)
		assert.NotNil(t, client.queueURLCache)
	})

	t.Run("with config", func(t *testing.T) {
		cfg := &Config{
			Region:          "us-east-1",
			Endpoint:        "http://localhost:4566",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
			SessionToken:    "test-token",
		}
		client := New(cfg)
		require.NotNil(t, client)
		assert.Equal(t, "us-east-1", client.cfg.Region)
		assert.Equal(t, "http://localhost:4566", client.cfg.Endpoint)
		assert.Equal(t, "test-key", client.cfg.AccessKeyID)
		assert.Equal(t, "test-secret", client.cfg.SecretAccessKey)
		assert.Equal(t, "test-token", client.cfg.SessionToken)
	})
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

	// Test with nil
	client.UseLogger(nil)
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

	// Test with nil
	client.UseMetrics(nil)
	assert.Equal(t, metrics, client.metrics)
}

func TestClient_UseTracer(t *testing.T) {
	client := New(&Config{})

	// Test with invalid type
	client.UseTracer("invalid")
	assert.Nil(t, client.tracer)

	// Test with nil
	client.UseTracer(nil)
	assert.Nil(t, client.tracer)
}

func TestClient_Connect_NoRegion(t *testing.T) {
	client := New(&Config{})
	client.UseLogger(NewMockLogger())

	client.Connect()

	// Client should not be connected without region
	assert.Nil(t, client.client)
}

func TestClient_Connect_NoLogger(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})

	client.Connect()

	// Client should not connect without logger
	assert.Nil(t, client.client)
}

func TestClient_isConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	// Not connected initially
	assert.False(t, client.isConnected())
}

func TestClient_Publish_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	err := client.Publish(context.Background(), "test-queue", []byte("test message"))
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestClient_Publish_EmptyTopic(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	err := client.Publish(context.Background(), "", []byte("test message"))
	assert.ErrorIs(t, err, errClientNotConnected) // Fails because client is nil
}

func TestClient_Subscribe_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to avoid waiting

	msg, err := client.Subscribe(ctx, "test-queue")

	require.ErrorIs(t, err, errClientNotConnected)
	assert.Nil(t, msg)
}

func TestClient_Subscribe_EmptyTopic(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())
	client.UseMetrics(NewMockMetrics())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg, err := client.Subscribe(ctx, "")

	require.ErrorIs(t, err, errClientNotConnected) // Fails because client is nil
	assert.Nil(t, msg)
}

func TestClient_CreateTopic_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.CreateTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestClient_CreateTopic_EmptyName(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.CreateTopic(context.Background(), "")
	assert.ErrorIs(t, err, errClientNotConnected) // Fails because client is nil
}

func TestClient_DeleteTopic_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.DeleteTopic(context.Background(), "test-queue")
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestClient_DeleteTopic_EmptyName(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.DeleteTopic(context.Background(), "")
	assert.ErrorIs(t, err, errClientNotConnected) // Fails because client is nil
}

func TestClient_Query_NotConnected(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	result, err := client.Query(context.Background(), "test-queue")

	require.ErrorIs(t, err, errClientNotConnected)
	assert.Nil(t, result)
}

func TestClient_Query_EmptyQuery(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	result, err := client.Query(context.Background(), "")

	require.ErrorIs(t, err, errClientNotConnected) // Fails because client is nil
	assert.Nil(t, result)
}

func TestClient_Close(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})
	client.UseLogger(NewMockLogger())

	err := client.Close()

	require.NoError(t, err)
	assert.False(t, client.isConnected())
}

func TestParseQueryArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected int32
	}{
		{
			name:     "no args",
			args:     nil,
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "empty args",
			args:     []any{},
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "valid int32 limit 1",
			args:     []any{int32(1)},
			expected: 1,
		},
		{
			name:     "valid int32 limit 5",
			args:     []any{int32(5)},
			expected: 5,
		},
		{
			name:     "valid int32 limit 10",
			args:     []any{int32(10)},
			expected: 10,
		},
		{
			name:     "int32 limit exceeds max",
			args:     []any{int32(15)},
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "invalid type int",
			args:     []any{5},
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "invalid type string",
			args:     []any{"invalid"},
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "int32 zero",
			args:     []any{int32(0)},
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "int32 negative",
			args:     []any{int32(-1)},
			expected: defaultQueryMaxMessages,
		},
		{
			name:     "multiple args uses first",
			args:     []any{int32(5), int32(3)},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQueryArgs(tt.args...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsContextCanceled(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "wrapped context canceled",
			err:      errWrappedContextCanceled,
			expected: true,
		},
		{
			name:     "wrapped deadline exceeded",
			err:      errWrappedDeadlineExceeded,
			expected: true,
		},
		{
			name:     "canceled keyword",
			err:      errRequestCanceled,
			expected: true,
		},
		{
			name:     "other error",
			err:      errClientNotConnected,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errGeneric,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContextCanceled(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      errConnectionRefused,
			expected: true,
		},
		{
			name:     "no such host",
			err:      errNoSuchHost,
			expected: true,
		},
		{
			name:     "network unreachable",
			err:      errNetworkUnreachable,
			expected: true,
		},
		{
			name:     "max attempts exceeded",
			err:      errMaxAttemptsExceeded,
			expected: true,
		},
		{
			name:     "queue not found",
			err:      errQueueNotFound,
			expected: false,
		},
		{
			name:     "client not connected",
			err:      errClientNotConnected,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errGeneric,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_startTrace(t *testing.T) {
	client := New(&Config{Region: "us-east-1"})

	ctx, span := client.startTrace(context.Background(), "test-span")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	assert.Implements(t, (*trace.Span)(nil), span)

	span.End()
}

func TestErrors(t *testing.T) {
	// Test error messages
	assert.Equal(t, "sqs client not connected", errClientNotConnected.Error())
	assert.Equal(t, "sqs queue not found", errQueueNotFound.Error())
	assert.Equal(t, "queue name cannot be empty", errEmptyQueueName.Error())
}
