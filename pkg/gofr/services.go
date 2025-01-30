package gofr

import (
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/container"
)

// AddOpenai sets the Openai wrapper in the app's container.
func (a *App) AddOpenai(openai container.OpenaiProvider) {
	openai.UseLogger(a.Logger())
	openai.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-openai")
	openai.UseTracer(tracer)

	openai.InitMetrics()

	a.container.Openai = openai
}
