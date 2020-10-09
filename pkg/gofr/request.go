package gofr

import (
	"context"
)

type Request interface {
	Context() context.Context
	Param(string) string
	// PathParam(string) string
	// Bind(interface{}) error
}
