package gofr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
