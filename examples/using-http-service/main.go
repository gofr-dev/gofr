package main

import (
	"io"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	a := gofr.New()

	// HTTP service with Circuit Breaker config given, uses custom health check
	// either of circuit breaker or health can be used as well, as both implement service.Options interface.
	// Note: /breeds is not an actual health check endpoint for "https://catfact.ninja"
	a.AddHTTPService("cat-facts", "https://catfact.ninja",
		&service.CircuitBreakerConfig{
			Threshold: 4,
			Interval:  1 * time.Second,
		},
		&service.HealthConfig{
			HealthEndpoint: "breeds",
		},
	)

	a.AddHTTPService("test-service", "http://localhost:9000",
		&service.Authentication{UserName: "abc", Password: "pass"})

	// service with improper health-check to test health check
	a.AddHTTPService("fact-checker", "https://catfact.ninja",
		&service.HealthConfig{
			HealthEndpoint: "breed",
		},
	)

	a.GET("/fact", Handler)

	a.Run()
}

func Handler(c *gofr.Context) (any, error) {
	//var data = struct {
	//	Fact   string `json:"fact"`
	//	Length int    `json:"length"`
	//}{}
	//
	//var catFacts = c.GetHTTPService("cat-facts")
	//
	//resp, err := catFacts.Get(c, "fact", map[string]interface{}{
	//	"max_length": 20,
	//})
	//if err != nil {
	//	return nil, err
	//}
	//
	//b, _ := io.ReadAll(resp.Body)
	//err = json.Unmarshal(b, &data)
	//if err != nil {
	//	return nil, err
	//}
	//
	//return data, nil

	var testService = c.GetHTTPService("test-service")
	resp, err := testService.Get(c, "auth", nil)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return string(b), nil
}
