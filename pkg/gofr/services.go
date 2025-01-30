package gofr

import (
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/container"
)

// AddOpenai sets the Openai wrapper in the app's container.
func (a *App) AddOpenAI(openAI container.OpenAIProvider) {
	openAI.UseLogger(a.Logger())
	openAI.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-openAI")
	openAI.UseTracer(tracer)

	openAI.InitMetrics()

	a.container.OpenAI = openAI
}
