package container

import (
	"context"
)

// OpenAI is the interface that wraps the basic endpoint of OpenAI API.

type OpenAI interface {
	// implementation of chat endpoint of OpenAI API
	CreateCompletions(ctx context.Context, r any) (any, error)
}

type OpenAIProvider interface {
	OpenAI

	// UseLogger set the logger for OpenAI client
	UseLogger(logger any)

	// UseMetrics set the logger for OpenAI client
	UseMetrics(metrics any)

	// UseTracer set the logger for OpenAI client
	UseTracer(tracer any)

	// InitMetrics is used to initializes metrics for the client
	InitMetrics()
}
