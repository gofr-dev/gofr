package gofr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
)

func TestShutdownWithContext_ContextTimeout(t *testing.T) {
	// Mock shutdown function that never completes
	mockShutdownFunc := func(ctx context.Context) error {
		// Simulate a long-running process
		<-ctx.Done()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := ShutdownWithContext(ctx, mockShutdownFunc, nil)

	require.ErrorIs(t, err, context.DeadlineExceeded, "Expected context deadline exceeded error")
}

func TestShutdownWithContext_SuccessfulShutdown(t *testing.T) {
	// Mock shutdown function that completes successfully
	mockShutdownFunc := func(_ context.Context) error {
		// Simulate a quick shutdown
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ShutdownWithContext(ctx, mockShutdownFunc, nil)

	require.NoError(t, err, "Expected successful shutdown without error")
}

func Test_getShutdownTimeoutFromConfig_Success(t *testing.T) {
	tests := []struct {
		name          string
		configValue   string
		expectedValue time.Duration
	}{
		{"Valid timeout", "1s", 1 * time.Second},
		{"Empty timeout", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(map[string]string{
				"SERVER_SHUTDOWN_THRESHOLD": tt.configValue,
			})

			timeout, err := getShutdownTimeoutFromConfig(mockConfig)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedValue, timeout)
		})
	}
}

func Test_getShutdownTimeoutFromConfig_Error(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"SERVER_SHUTDOWN_THRESHOLD": "invalid",
	})

	_, err := getShutdownTimeoutFromConfig(mockConfig)
	require.Error(t, err)
}
