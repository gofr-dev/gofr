package gofr

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/config"
)

// ShutdownWithContext handles the shutdown process with context timeout.
// It takes a shutdown function and a force close function as parameters.
// If the context times out, the force close function is called.
func ShutdownWithContext(ctx context.Context, shutdownFunc func(ctx context.Context) error, forceCloseFunc func() error) error {
	errCh := make(chan error, 1) // Channel to receive shutdown error

	go func() {
		errCh <- shutdownFunc(ctx) // Run shutdownFunc in a goroutine and send any error to errCh
	}()

	// Wait for either the context to be done or shutdownFunc to complete
	select {
	case <-ctx.Done(): // Context timeout reached
		err := ctx.Err()

		if forceCloseFunc != nil {
			err = errors.Join(err, forceCloseFunc()) // Attempt force close if available
		}

		return err
	case err := <-errCh:
		return err
	}
}

func getShutdownTimeoutFromConfig(cfg config.Config) (time.Duration, error) {
	value := cfg.GetOrDefault("SHUTDOWN_GRACE_PERIOD", "30s")
	if value == "" {
		return shutDownTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return shutDownTimeout, err
	}

	return timeout, nil
}
