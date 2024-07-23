package gofr

import (
	"context"
	"errors"
)

// shutdownWithContext handles the shutdown process with context timeout.
func shutdownWithContext(ctx context.Context, shutdownFunc func(ctx context.Context) error, forceCloseFunc func() error) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- shutdownFunc(ctx)
	}()
	select {
	case <-ctx.Done():
		err := ctx.Err()

		if forceCloseFunc != nil {
			err = errors.Join(err, forceCloseFunc())
		}

		return err
	case err := <-errCh:
		return err
	}
}
