package gofr

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/version"
)

// StartFunc defines a function that is executed when the application starts.
// Returning an error will stop the application from starting the servers.
type StartFunc func(ctx *Context) error

// startJob represents a startup task registered with the App.
type startJob struct {
	name string
	fn   StartFunc
}

func (a *App) runStartJobs() error {
	for _, j := range a.startJobs {
		ctx, span := otel.GetTracerProvider().Tracer("gofr-"+version.Framework).Start(context.Background(), j.name)
		logger := logging.NewContextLogger(ctx, a.container.Logger)
		c := &Context{
			Context:       ctx,
			Container:     a.container,
			Request:       noopRequest{},
			ContextLogger: *logger,
		}

		c.Infof("Starting startup job: %s", j.name)
		start := time.Now()
		err := func() (err error) {
			defer span.End()
			defer func() {
				if r := recover(); r != nil {
					c.Errorf("Panic in startup job %s: %v", j.name, r)
					if err == nil {
						err = fmt.Errorf("%v", r)
					}
				}
				c.Infof("Finished startup job: %s in %s", j.name, time.Since(start))
			}()
			err = j.fn(c)
			return
		}()
		if err != nil {
			return fmt.Errorf("startup job %s failed: %w", j.name, err)
		}
	}

	return nil
}

// AddStartJob registers a synchronous job to be executed before the servers start.
func (a *App) AddStartJob(name string, fn StartFunc) {
	if fn == nil {
		a.Logger().Errorf("nil function provided for startup job: %s", name)
		return
	}
	a.startJobs = append(a.startJobs, startJob{name: name, fn: fn})
}
