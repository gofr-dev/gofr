package main

import "gofr.dev/pkg/gofr"

func main() {
	app := gofr.New()

	// Simple greeting endpoint
	app.GET("/", func(ctx *gofr.Context) (any, error) {
		return map[string]string{
			"message": "Welcome to GoFr!",
			"status":  "running",
			"version": "1.0.0",
		}, nil
	})

	// Health check endpoint
	app.GET("/health", func(ctx *gofr.Context) (any, error) {
		return map[string]string{
			"status": "healthy",
		}, nil
	})

	// Echo endpoint
	app.GET("/echo/{message}", func(ctx *gofr.Context) (any, error) {
		message := ctx.PathParam("message")
		return map[string]string{
			"echo": message,
		}, nil
	})

	// Start the server (default port: 8000)
	app.Run()
}
