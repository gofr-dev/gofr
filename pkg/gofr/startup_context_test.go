package gofr

import (
	"context"
	"reflect"
	"testing"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

func TestStartupContext_Construction(t *testing.T) {
	c := &container.Container{}
	baseLogger := logging.NewLogger(logging.DEBUG)
	ctx := context.Background()
	ctxLogger := logging.NewContextLogger(ctx, baseLogger)

	sc := StartupContext{
		Context:       ctx,
		Container:     c,
		ContextLogger: *ctxLogger,
	}

	if sc.Container != c {
		t.Errorf("expected container to be set")
	}
	if reflect.TypeOf(sc.ContextLogger).String() != "logging.ContextLogger" {
		t.Errorf("expected logger to be of type ContextLogger")
	}
	if sc.Context != ctx {
		t.Errorf("expected context to be set")
	}
}
