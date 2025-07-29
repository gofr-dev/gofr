package main

import (
	"encoding/json"
	"io"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	a := gofr.New()

	a.AddHTTPService("cat-facts", "https://catfact.ninja",
		&service.CircuitBreakerConfig{
			Threshold: 4,
			Interval:  1 * time.Second,
		},
		&service.HealthConfig{
			HealthEndpoint: "breeds",
		},
		// ADDED: RateLimiterConfig for the "cat-facts" service
		// Set a low RPS and Burst to easily trigger the rate limit for testing
		&service.WithRateLimiter{
			Config: service.RateLimiterConfig{
				RequestsPerSecond: 1, // Allow 1 request per second
				Burst:             1, // Allow a burst of 1 request
			},
			// Logger, Metrics, and ServiceURL will be injected by NewHTTPService
		},
	)

	a.AddHTTPService("fact-checker", "https://catfact.ninja",
		&service.HealthConfig{
			HealthEndpoint: "breed",
		},
	)

	a.GET("/fact", Handler)

	a.Run()
}

func Handler(c *gofr.Context) (any, error) {
	var data = struct {
		Fact   string `json:"fact"`
		Length int    `json:"length"`
	}{}

	var catFacts = c.GetHTTPService("cat-facts")

	resp, err := catFacts.Get(c, "fact", map[string]any{
		"max_length": 20,
	})
	if err != nil {
		return nil, err
	}

	b, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
