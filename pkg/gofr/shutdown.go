package gofr

import (
	"context"
)

// shutdownWithTimeout handles the shutdown process with context timeout.
func shutdownWithTimeout(ctx context.Context, shutdownFunc func(ctx context.Context) error, forceCloseFunc func()) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- shutdownFunc(ctx)
	}()

	select {
	case <-ctx.Done():
		if forceCloseFunc != nil {
			forceCloseFunc()
		}

		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
