package main

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	app := gofr.New()

	// HTTP service with optimized connection pool for high-frequency requests
	app.AddHTTPService("api-service", "https://jsonplaceholder.typicode.com",
		&service.ConnectionPoolConfig{
			MaxIdleConns:        100, // Maximum idle connections across all hosts
			MaxIdleConnsPerHost: 20,  // Maximum idle connections per host (increased from default 2)
			IdleConnTimeout:     90 * time.Second, // Keep connections alive for 90 seconds
		},
	)

	app.GET("/posts/{id}", func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")
		
		svc := ctx.GetHTTPService("api-service")
		resp, err := svc.Get(ctx, "posts/"+id, nil)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		return map[string]any{
			"status": resp.Status,
			"headers": resp.Header,
		}, nil
	})

	app.Run()
}