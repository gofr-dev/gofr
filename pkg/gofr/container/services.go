package container

import (
	"context"
)

// Openai is the interface that wraps the basic endpoint of openai api.

type OpenAI interface {
	// implementation of chat endpoint of openai api
	CreateCompletions(ctx context.Context, r any) (any, error)
}

type OpenAIProvider interface {
	OpenAI

	// UseLogger set the logger for openai client
	UseLogger(logger any)

	// UseMetrics set the logger for openai client
	UseMetrics(metrics any)

	// UseTracer set the logger for openai client
	UseTracer(tracer any)

	// InitMetrics is used to initializes metrics for the client
	InitMetrics()
}
