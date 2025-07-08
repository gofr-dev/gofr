package gofr

import (
	"context"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

type StartupContext struct {
	context.Context
	*container.Container
	logging.ContextLogger
}
