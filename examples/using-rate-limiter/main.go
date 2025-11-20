package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/middleware"
)

func main() {
	app := gofr.New()

	// Configure rate limiter: 5 requests per second with burst of 10
	// This limits each IP to 5 req/sec on average, allowing bursts up to 10
	rateLimiterConfig := middleware.RateLimiterConfig{
		RequestsPerSecond: 5,
		Burst:             10,
		PerIP:             true, // Enable per-IP rate limiting
	}

	// Add rate limiter middleware
	app.UseMiddleware(middleware.RateLimiter(rateLimiterConfig, app.Metrics()))

	// Define routes
	app.GET("/limited", limitedHandler)
	app.GET("/test", testHandler)

	app.Run()
}

func limitedHandler(c *gofr.Context) (any, error) {
	return map[string]string{
		"message": "This endpoint is rate limited to 5 req/sec per IP",
		"tip":     "Try sending multiple rapid requests to see 429 errors",
	}, nil
}

func testHandler(c *gofr.Context) (any, error) {
	return map[string]string{
		"message": "Test endpoint also rate limited",
		"status":  "success",
	}, nil
}
