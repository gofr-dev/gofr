package main

import (
	"gofr.dev/examples/using-pubsub/handlers"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	err := app.NewHistogram(handlers.PublishEventHistogram,
		"Histogram for time taken to publish event in seconds",
		[]float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30})
	if err != nil {
		app.Logger.Warnf("error while creating histogram, %v", err)
	}

	err = app.NewSummary(handlers.ConsumeEventSummary,
		"Summary for time taken to consume event in seconds")
	if err != nil {
		app.Logger.Warnf("error while creating summary, %v", err)
	}

	app.GET("/pub", handlers.Producer)
	app.GET("/sub", handlers.Consumer)
	app.GET("/subCommit", handlers.ConsumerWithCommit)

	app.Server.HTTP.Port = 9112
	app.Start()
}
