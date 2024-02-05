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

	a.GET("/service", func(c *gofr.Context) (interface{}, error) {
		var data = struct {
			Fact   string `json:"fact"`
			Length int    `json:"length"`
		}{}

		var service1 = c.GetHTTPService("service1")
		resp, err := service1.Get(c, "fact", map[string]interface{}{
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
	})
	a.POST("/metrics", func(c *gofr.Context) (interface{}, error) {
		var data = make(map[string]string)
		err := c.Request.Bind(&data)
		if err != nil {
			c.Logger.Errorf("error : %v", err)
		}

		c.Logger.Info(data)

		return nil, nil
	})

	// HTTP service with Circuit Breaker config given, uses default health check
	// Note: /breeds is not an actual health check endpoint for "https://catfact.ninja"
	a.AddHTTPService("service1", "https://catfact.ninja",
		&service.CircuitBreakerConfig{
			Threshold: 4,
			Timeout:   5 * time.Second,
			Interval:  1 * time.Second,
		},
		&service.HealthConfig{
			HealthEndpoint: "breeds",
		},
	)

	// HTTP service with Health check config for custom health check endpoint
	// Note: The health endpoint here /breed for "https://catfact.ninja" will give 404
	a.AddHTTPService("service2", "https://catfact.ninja",
		&service.HealthConfig{
			HealthEndpoint: "breed",
		},
	)

	a.Run()
}
