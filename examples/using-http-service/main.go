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

	// HTTP service with Circuit Breaker, Health Check, and Connection Pool configuration
	// Note: /breeds is not an actual health check endpoint for "https://catfact.ninja"
	a.AddHTTPService("cat-facts", "https://catfact.ninja",
		&service.CircuitBreakerConfig{
			Threshold: 4,
			Interval:  1 * time.Second,
		},
		&service.HealthConfig{
			HealthEndpoint: "breeds",
		},
		&service.ConnectionPoolConfig{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	)

	// service with connection pool configuration for high-frequency requests
	a.AddHTTPService("fact-checker", "https://catfact.ninja",
		&service.HealthConfig{
			HealthEndpoint: "breed",
		},
		&service.ConnectionPoolConfig{
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     15 * time.Second,
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
